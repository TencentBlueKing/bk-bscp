// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package gen

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"gorm.io/plugin/dbresolver"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func newTemplateVariable(db *gorm.DB, opts ...gen.DOOption) templateVariable {
	_templateVariable := templateVariable{}

	_templateVariable.templateVariableDo.UseDB(db, opts...)
	_templateVariable.templateVariableDo.UseModel(&table.TemplateVariable{})

	tableName := _templateVariable.templateVariableDo.TableName()
	_templateVariable.ALL = field.NewAsterisk(tableName)
	_templateVariable.ID = field.NewUint32(tableName, "id")
	_templateVariable.Name = field.NewString(tableName, "name")
	_templateVariable.Type = field.NewString(tableName, "type")
	_templateVariable.DefaultVal = field.NewString(tableName, "default_val")
	_templateVariable.Memo = field.NewString(tableName, "memo")
	_templateVariable.BizID = field.NewUint32(tableName, "biz_id")
	_templateVariable.Creator = field.NewString(tableName, "creator")
	_templateVariable.Reviser = field.NewString(tableName, "reviser")
	_templateVariable.CreatedAt = field.NewTime(tableName, "created_at")
	_templateVariable.UpdatedAt = field.NewTime(tableName, "updated_at")

	_templateVariable.fillFieldMap()

	return _templateVariable
}

type templateVariable struct {
	templateVariableDo templateVariableDo

	ALL        field.Asterisk
	ID         field.Uint32
	Name       field.String
	Type       field.String
	DefaultVal field.String
	Memo       field.String
	BizID      field.Uint32
	Creator    field.String
	Reviser    field.String
	CreatedAt  field.Time
	UpdatedAt  field.Time

	fieldMap map[string]field.Expr
}

func (t templateVariable) Table(newTableName string) *templateVariable {
	t.templateVariableDo.UseTable(newTableName)
	return t.updateTableName(newTableName)
}

func (t templateVariable) As(alias string) *templateVariable {
	t.templateVariableDo.DO = *(t.templateVariableDo.As(alias).(*gen.DO))
	return t.updateTableName(alias)
}

func (t *templateVariable) updateTableName(table string) *templateVariable {
	t.ALL = field.NewAsterisk(table)
	t.ID = field.NewUint32(table, "id")
	t.Name = field.NewString(table, "name")
	t.Type = field.NewString(table, "type")
	t.DefaultVal = field.NewString(table, "default_val")
	t.Memo = field.NewString(table, "memo")
	t.BizID = field.NewUint32(table, "biz_id")
	t.Creator = field.NewString(table, "creator")
	t.Reviser = field.NewString(table, "reviser")
	t.CreatedAt = field.NewTime(table, "created_at")
	t.UpdatedAt = field.NewTime(table, "updated_at")

	t.fillFieldMap()

	return t
}

func (t *templateVariable) WithContext(ctx context.Context) ITemplateVariableDo {
	return t.templateVariableDo.WithContext(ctx)
}

func (t templateVariable) TableName() string { return t.templateVariableDo.TableName() }

func (t templateVariable) Alias() string { return t.templateVariableDo.Alias() }

func (t templateVariable) Columns(cols ...field.Expr) gen.Columns {
	return t.templateVariableDo.Columns(cols...)
}

func (t *templateVariable) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := t.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (t *templateVariable) fillFieldMap() {
	t.fieldMap = make(map[string]field.Expr, 10)
	t.fieldMap["id"] = t.ID
	t.fieldMap["name"] = t.Name
	t.fieldMap["type"] = t.Type
	t.fieldMap["default_val"] = t.DefaultVal
	t.fieldMap["memo"] = t.Memo
	t.fieldMap["biz_id"] = t.BizID
	t.fieldMap["creator"] = t.Creator
	t.fieldMap["reviser"] = t.Reviser
	t.fieldMap["created_at"] = t.CreatedAt
	t.fieldMap["updated_at"] = t.UpdatedAt
}

func (t templateVariable) clone(db *gorm.DB) templateVariable {
	t.templateVariableDo.ReplaceConnPool(db.Statement.ConnPool)
	return t
}

func (t templateVariable) replaceDB(db *gorm.DB) templateVariable {
	t.templateVariableDo.ReplaceDB(db)
	return t
}

type templateVariableDo struct{ gen.DO }

type ITemplateVariableDo interface {
	gen.SubQuery
	Debug() ITemplateVariableDo
	WithContext(ctx context.Context) ITemplateVariableDo
	WithResult(fc func(tx gen.Dao)) gen.ResultInfo
	ReplaceDB(db *gorm.DB)
	ReadDB() ITemplateVariableDo
	WriteDB() ITemplateVariableDo
	As(alias string) gen.Dao
	Session(config *gorm.Session) ITemplateVariableDo
	Columns(cols ...field.Expr) gen.Columns
	Clauses(conds ...clause.Expression) ITemplateVariableDo
	Not(conds ...gen.Condition) ITemplateVariableDo
	Or(conds ...gen.Condition) ITemplateVariableDo
	Select(conds ...field.Expr) ITemplateVariableDo
	Where(conds ...gen.Condition) ITemplateVariableDo
	Order(conds ...field.Expr) ITemplateVariableDo
	Distinct(cols ...field.Expr) ITemplateVariableDo
	Omit(cols ...field.Expr) ITemplateVariableDo
	Join(table schema.Tabler, on ...field.Expr) ITemplateVariableDo
	LeftJoin(table schema.Tabler, on ...field.Expr) ITemplateVariableDo
	RightJoin(table schema.Tabler, on ...field.Expr) ITemplateVariableDo
	Group(cols ...field.Expr) ITemplateVariableDo
	Having(conds ...gen.Condition) ITemplateVariableDo
	Limit(limit int) ITemplateVariableDo
	Offset(offset int) ITemplateVariableDo
	Count() (count int64, err error)
	Scopes(funcs ...func(gen.Dao) gen.Dao) ITemplateVariableDo
	Unscoped() ITemplateVariableDo
	Create(values ...*table.TemplateVariable) error
	CreateInBatches(values []*table.TemplateVariable, batchSize int) error
	Save(values ...*table.TemplateVariable) error
	First() (*table.TemplateVariable, error)
	Take() (*table.TemplateVariable, error)
	Last() (*table.TemplateVariable, error)
	Find() ([]*table.TemplateVariable, error)
	FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*table.TemplateVariable, err error)
	FindInBatches(result *[]*table.TemplateVariable, batchSize int, fc func(tx gen.Dao, batch int) error) error
	Pluck(column field.Expr, dest interface{}) error
	Delete(...*table.TemplateVariable) (info gen.ResultInfo, err error)
	Update(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	Updates(value interface{}) (info gen.ResultInfo, err error)
	UpdateColumn(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateColumnSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	UpdateColumns(value interface{}) (info gen.ResultInfo, err error)
	UpdateFrom(q gen.SubQuery) gen.Dao
	Attrs(attrs ...field.AssignExpr) ITemplateVariableDo
	Assign(attrs ...field.AssignExpr) ITemplateVariableDo
	Joins(fields ...field.RelationField) ITemplateVariableDo
	Preload(fields ...field.RelationField) ITemplateVariableDo
	FirstOrInit() (*table.TemplateVariable, error)
	FirstOrCreate() (*table.TemplateVariable, error)
	FindByPage(offset int, limit int) (result []*table.TemplateVariable, count int64, err error)
	ScanByPage(result interface{}, offset int, limit int) (count int64, err error)
	Scan(result interface{}) (err error)
	Returning(value interface{}, columns ...string) ITemplateVariableDo
	UnderlyingDB() *gorm.DB
	schema.Tabler
}

func (t templateVariableDo) Debug() ITemplateVariableDo {
	return t.withDO(t.DO.Debug())
}

func (t templateVariableDo) WithContext(ctx context.Context) ITemplateVariableDo {
	return t.withDO(t.DO.WithContext(ctx))
}

func (t templateVariableDo) ReadDB() ITemplateVariableDo {
	return t.Clauses(dbresolver.Read)
}

func (t templateVariableDo) WriteDB() ITemplateVariableDo {
	return t.Clauses(dbresolver.Write)
}

func (t templateVariableDo) Session(config *gorm.Session) ITemplateVariableDo {
	return t.withDO(t.DO.Session(config))
}

func (t templateVariableDo) Clauses(conds ...clause.Expression) ITemplateVariableDo {
	return t.withDO(t.DO.Clauses(conds...))
}

func (t templateVariableDo) Returning(value interface{}, columns ...string) ITemplateVariableDo {
	return t.withDO(t.DO.Returning(value, columns...))
}

func (t templateVariableDo) Not(conds ...gen.Condition) ITemplateVariableDo {
	return t.withDO(t.DO.Not(conds...))
}

func (t templateVariableDo) Or(conds ...gen.Condition) ITemplateVariableDo {
	return t.withDO(t.DO.Or(conds...))
}

func (t templateVariableDo) Select(conds ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.Select(conds...))
}

func (t templateVariableDo) Where(conds ...gen.Condition) ITemplateVariableDo {
	return t.withDO(t.DO.Where(conds...))
}

func (t templateVariableDo) Order(conds ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.Order(conds...))
}

func (t templateVariableDo) Distinct(cols ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.Distinct(cols...))
}

func (t templateVariableDo) Omit(cols ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.Omit(cols...))
}

func (t templateVariableDo) Join(table schema.Tabler, on ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.Join(table, on...))
}

func (t templateVariableDo) LeftJoin(table schema.Tabler, on ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.LeftJoin(table, on...))
}

func (t templateVariableDo) RightJoin(table schema.Tabler, on ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.RightJoin(table, on...))
}

func (t templateVariableDo) Group(cols ...field.Expr) ITemplateVariableDo {
	return t.withDO(t.DO.Group(cols...))
}

func (t templateVariableDo) Having(conds ...gen.Condition) ITemplateVariableDo {
	return t.withDO(t.DO.Having(conds...))
}

func (t templateVariableDo) Limit(limit int) ITemplateVariableDo {
	return t.withDO(t.DO.Limit(limit))
}

func (t templateVariableDo) Offset(offset int) ITemplateVariableDo {
	return t.withDO(t.DO.Offset(offset))
}

func (t templateVariableDo) Scopes(funcs ...func(gen.Dao) gen.Dao) ITemplateVariableDo {
	return t.withDO(t.DO.Scopes(funcs...))
}

func (t templateVariableDo) Unscoped() ITemplateVariableDo {
	return t.withDO(t.DO.Unscoped())
}

func (t templateVariableDo) Create(values ...*table.TemplateVariable) error {
	if len(values) == 0 {
		return nil
	}
	return t.DO.Create(values)
}

func (t templateVariableDo) CreateInBatches(values []*table.TemplateVariable, batchSize int) error {
	return t.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (t templateVariableDo) Save(values ...*table.TemplateVariable) error {
	if len(values) == 0 {
		return nil
	}
	return t.DO.Save(values)
}

func (t templateVariableDo) First() (*table.TemplateVariable, error) {
	if result, err := t.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateVariable), nil
	}
}

func (t templateVariableDo) Take() (*table.TemplateVariable, error) {
	if result, err := t.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateVariable), nil
	}
}

func (t templateVariableDo) Last() (*table.TemplateVariable, error) {
	if result, err := t.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateVariable), nil
	}
}

func (t templateVariableDo) Find() ([]*table.TemplateVariable, error) {
	result, err := t.DO.Find()
	return result.([]*table.TemplateVariable), err
}

func (t templateVariableDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*table.TemplateVariable, err error) {
	buf := make([]*table.TemplateVariable, 0, batchSize)
	err = t.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (t templateVariableDo) FindInBatches(result *[]*table.TemplateVariable, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return t.DO.FindInBatches(result, batchSize, fc)
}

func (t templateVariableDo) Attrs(attrs ...field.AssignExpr) ITemplateVariableDo {
	return t.withDO(t.DO.Attrs(attrs...))
}

func (t templateVariableDo) Assign(attrs ...field.AssignExpr) ITemplateVariableDo {
	return t.withDO(t.DO.Assign(attrs...))
}

func (t templateVariableDo) Joins(fields ...field.RelationField) ITemplateVariableDo {
	for _, _f := range fields {
		t = *t.withDO(t.DO.Joins(_f))
	}
	return &t
}

func (t templateVariableDo) Preload(fields ...field.RelationField) ITemplateVariableDo {
	for _, _f := range fields {
		t = *t.withDO(t.DO.Preload(_f))
	}
	return &t
}

func (t templateVariableDo) FirstOrInit() (*table.TemplateVariable, error) {
	if result, err := t.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateVariable), nil
	}
}

func (t templateVariableDo) FirstOrCreate() (*table.TemplateVariable, error) {
	if result, err := t.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateVariable), nil
	}
}

func (t templateVariableDo) FindByPage(offset int, limit int) (result []*table.TemplateVariable, count int64, err error) {
	result, err = t.Offset(offset).Limit(limit).Find()
	if err != nil {
		return
	}

	if size := len(result); 0 < limit && 0 < size && size < limit {
		count = int64(size + offset)
		return
	}

	count, err = t.Offset(-1).Limit(-1).Count()
	return
}

func (t templateVariableDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = t.Count()
	if err != nil {
		return
	}

	err = t.Offset(offset).Limit(limit).Scan(result)
	return
}

func (t templateVariableDo) Scan(result interface{}) (err error) {
	return t.DO.Scan(result)
}

func (t templateVariableDo) Delete(models ...*table.TemplateVariable) (result gen.ResultInfo, err error) {
	return t.DO.Delete(models)
}

func (t *templateVariableDo) withDO(do gen.Dao) *templateVariableDo {
	t.DO = *do.(*gen.DO)
	return t
}
