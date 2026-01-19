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

// Package pushmanager provides bcs push manager api client.
package pushmanager

import (
	"errors"
	"fmt"
	"strings"
)

func (r *CreatePushEventRequest) validate() error {
	if r == nil || r.Event.Domain == "" {
		return errors.New("event.domain is required")
	}

	fields := r.Event.EventDetail.Fields
	types := fields.Types
	if types == "" {
		return errors.New("event.event_detail.fields.types is required")
	}

	if strings.Contains(types, string(PushTypeRTX)) {
		if fields.RTXReceivers == "" || fields.RTXTitle == "" || fields.RTXContent == "" {
			return errors.New("rtx requires rtx_receivers, rtx_title and rtx_content")
		}
	}

	if strings.Contains(types, string(PushTypeMail)) {
		if fields.MailReceivers == "" || fields.MailTitle == "" || fields.MailContent == "" {
			return errors.New("mail requires mail_receivers, mail_title and mail_content")
		}
	}

	if strings.Contains(types, string(PushTypeMsg)) {
		if fields.MsgReceivers == "" || fields.MsgContent == "" {
			return errors.New("msg requires msg_receivers and msg_content")
		}
	}

	if r.Event.Dimension != nil && len(r.Event.Dimension.Fields) == 0 {
		return errors.New("dimension.fields cannot be empty when dimension is provided")
	}

	if r.Event.MetricData != nil && r.Event.MetricData.Timestamp == "" {
		return errors.New("metric_data.timestamp is required when metric_data is provided")
	}

	return nil
}

func (r *CreatePushWhitelistRequest) validate() error {
	if r == nil {
		return errors.New("request is nil")
	}

	w := r.Whitelist
	if w.Domain == "" {
		return errors.New("whitelist.domain is required")
	}
	if len(w.Dimension.Fields) == 0 {
		return errors.New("whitelist.dimension.fields is required")
	}
	if w.StartTime == "" || w.EndTime == "" {
		return errors.New("start_time and end_time are required")
	}

	return nil
}

func (r *CreatePushTemplateRequest) validate() error {
	if r == nil {
		return errors.New("request is nil")
	}

	t := r.Template
	if t.TemplateID == "" || t.Domain == "" {
		return errors.New("template_id and domain are required")
	}

	if t.Content.Title == "" || t.Content.Body == "" {
		return errors.New("template.content.title and body are required")
	}

	return nil
}

func validateID(domain, id, name string) error {
	if domain == "" {
		return errors.New("domain is required")
	}
	if id == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
