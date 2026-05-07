/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package ratelimit 基于 Redis 固定窗口的组件级限流器，超限请求会延迟等待
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
)

// ErrRateLimited 表示请求因限流被拒绝或等待超时。
var ErrRateLimited = errors.New("component rate limited")

const (
	componentRateLimitSubsystem = "component_rate_limit"
)

var (
	metricOnce   sync.Once
	globalMetric *limiterMetric
)

type limiterMetric struct {
	acquireTotal    *prometheus.CounterVec
	redisErrorTotal *prometheus.CounterVec
	delayDuration   *prometheus.HistogramVec
}

func initMetric() *limiterMetric {
	metricOnce.Do(func() {
		globalMetric = &limiterMetric{
			acquireTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: metrics.Namespace,
				Subsystem: componentRateLimitSubsystem,
				Name:      "acquire_total",
				Help:      "Total acquire attempts of component rate limiter",
			}, []string{"component", "result"}),
			redisErrorTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: metrics.Namespace,
				Subsystem: componentRateLimitSubsystem,
				Name:      "redis_error_total",
				Help:      "Total Redis errors of component rate limiter",
			}, []string{"component"}),
			delayDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: metrics.Namespace,
				Subsystem: componentRateLimitSubsystem,
				Name:      "delay_duration_seconds",
				Help:      "Time spent waiting for rate limit window to reset",
				Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
			}, []string{"component"}),
		}
		metrics.Register().MustRegister(globalMetric.acquireTotal)
		metrics.Register().MustRegister(globalMetric.redisErrorTotal)
		metrics.Register().MustRegister(globalMetric.delayDuration)
	})
	return globalMetric
}

// fixedWindowLimiter 基于 Redis INCR 的固定窗口限流器。
type fixedWindowLimiter struct {
	cfg    cc.ComponentRateLimit
	redis  bedis.Client
	now    func() time.Time
	metric *limiterMetric
}

// New 根据配置创建限流器
func New(cfg cc.ComponentRateLimit, redisClient bedis.Client) (components.Limiter, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	return NewRedisLimiter(cfg, redisClient)
}

// NewRedisLimiter 创建基于 Redis 的固定窗口限流器。
func NewRedisLimiter(cfg cc.ComponentRateLimit, redisClient bedis.Client) (*fixedWindowLimiter, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if redisClient == nil {
		return nil, errors.New("redis client is required for component rate limit")
	}
	return &fixedWindowLimiter{
		cfg:    cfg,
		redis:  redisClient,
		now:    time.Now,
		metric: initMetric(),
	}, nil
}

// Acquire 尝试获取限流许可。超限时延迟等待到下一个窗口重试，最多等待 maxWaitSeconds。
func (l *fixedWindowLimiter) Acquire(ctx context.Context, component string) error {
	if l == nil || !l.cfg.Enabled {
		logs.Infof("component rate limit not enabled, component=%s", component)
		return nil
	}

	rule, ok := l.cfg.Components[component]
	if !ok || !rule.Enabled || rule.Limit == 0 {
		logs.Infof("component rate limit not enabled, component=%s", component)
		return nil
	}

	if ctx.Err() != nil {
		logs.Warnf("rate limit skip component=%s, context already expired: %v", component, ctx.Err())
		return ctx.Err()
	}

	maxWait := time.Duration(l.cfg.MaxWaitSeconds) * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	start := l.now()
	delayed := false

	for {
		err := l.acquireRedis(waitCtx, component, rule)

		if err == nil {
			if delayed {
				elapsed := l.now().Sub(start).Seconds()
				l.metric.delayDuration.WithLabelValues(component).Observe(elapsed)
				l.metric.acquireTotal.WithLabelValues(component, "delayed_allow").Inc()
				logs.Infof("rate limit delayed_allow component=%s, waited=%.2fs", component, elapsed)
			} else {
				l.metric.acquireTotal.WithLabelValues(component, "allow").Inc()
			}
			return nil
		}

		if errors.Is(err, ErrRateLimited) {
			delayed = true
			waitDur := l.timeUntilNextWindow()
			logs.Warnf("rate limit triggered component=%s limit=%d window=%ds, delaying %.2fs until next window",
				component, rule.Limit, l.cfg.WindowSeconds, waitDur.Seconds())

			timer := time.NewTimer(waitDur)
			select {
			case <-waitCtx.Done():
				timer.Stop()
				l.metric.acquireTotal.WithLabelValues(component, "timeout").Inc()
				logs.Errorf("rate limit timeout component=%s, waited=%.2fs, maxWait=%ds",
					component, l.now().Sub(start).Seconds(), l.cfg.MaxWaitSeconds)
				return fmt.Errorf("%w: max wait %ds exceeded", ErrRateLimited, l.cfg.MaxWaitSeconds)
			case <-timer.C:
				continue
			}
		}

		// 区分 context 过期和 Redis 真正异常
		if waitCtx.Err() != nil {
			l.metric.acquireTotal.WithLabelValues(component, "timeout").Inc()
			logs.Warnf("rate limit context expired component=%s, err: %v", component, waitCtx.Err())
			return waitCtx.Err()
		}

		// Redis 异常
		if l.cfg.FailOpenEnabled() {
			l.metric.acquireTotal.WithLabelValues(component, "fail_open").Inc()
			logs.Warnf("rate limit fail_open component=%s, redis err: %v", component, err)
			return nil
		}

		l.metric.acquireTotal.WithLabelValues(component, "error").Inc()
		logs.Errorf("rate limit error component=%s, err: %v", component, err)
		return err
	}
}

// acquireRedis 通过 Redis INCR 原子递增计数，首次创建 key 时设置 TTL。
func (l *fixedWindowLimiter) acquireRedis(ctx context.Context, component string,
	rule cc.ComponentRateLimitRule) error {

	key := l.windowKey(component, l.now())

	count, err := l.redis.Incr(ctx, key)
	if err != nil {
		l.metric.redisErrorTotal.WithLabelValues(component).Inc()
		return fmt.Errorf("increment rate limit key %s failed: %w", key, err)
	}
	if count == 1 {
		if err := l.redis.Expire(ctx, key, int(l.cfg.KeyTTLSeconds), ""); err != nil {
			l.metric.redisErrorTotal.WithLabelValues(component).Inc()
			return fmt.Errorf("expire rate limit key %s failed: %w", key, err)
		}
	}
	if uint(count) > rule.Limit {
		return fmt.Errorf("%w: component=%s limit=%d window=%ds count=%d",
			ErrRateLimited, component, rule.Limit, l.cfg.WindowSeconds, count)
	}
	return nil
}

// timeUntilNextWindow 计算当前时间到下一个限流窗口起点的剩余时间。
// 通过整数除法将当前时间向下对齐到窗口边界，再加一个窗口长度得到下一窗口起点。
func (l *fixedWindowLimiter) timeUntilNextWindow() time.Duration {
	now := l.now()
	windowStart := (now.Unix() / int64(l.cfg.WindowSeconds)) * int64(l.cfg.WindowSeconds)
	windowEnd := windowStart + int64(l.cfg.WindowSeconds)
	return time.Unix(windowEnd, 0).Sub(now)
}

func (l *fixedWindowLimiter) windowKey(component string, now time.Time) string {
	windowStart := (now.Unix() / int64(l.cfg.WindowSeconds)) * int64(l.cfg.WindowSeconds)
	return fmt.Sprintf("bscp:component_ratelimit:%s:%d", component, windowStart)
}

func validateConfig(cfg cc.ComponentRateLimit) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.WindowSeconds == 0 {
		return errors.New("component rate limit windowSeconds must be greater than 0")
	}
	if cfg.KeyTTLSeconds == 0 {
		return errors.New("component rate limit keyTTLSeconds must be greater than 0")
	}
	if cfg.KeyTTLSeconds < cfg.WindowSeconds {
		return errors.New("component rate limit keyTTLSeconds must be greater than or equal to windowSeconds")
	}
	for component, rule := range cfg.Components {
		if !rule.Enabled {
			continue
		}
		if rule.Limit == 0 {
			return fmt.Errorf("component rate limit %s limit must be greater than 0", component)
		}
	}
	return nil
}
