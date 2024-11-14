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

func newTemplateRevision(db *gorm.DB, opts ...gen.DOOption) templateRevision {
	_templateRevision := templateRevision{}

	_templateRevision.templateRevisionDo.UseDB(db, opts...)
	_templateRevision.templateRevisionDo.UseModel(&table.TemplateRevision{})

	tableName := _templateRevision.templateRevisionDo.TableName()
	_templateRevision.ALL = field.NewAsterisk(tableName)
	_templateRevision.ID = field.NewUint32(tableName, "id")
	_templateRevision.RevisionName = field.NewString(tableName, "revision_name")
	_templateRevision.RevisionMemo = field.NewString(tableName, "revision_memo")
	_templateRevision.Name = field.NewString(tableName, "name")
	_templateRevision.Path = field.NewString(tableName, "path")
	_templateRevision.FileType = field.NewString(tableName, "file_type")
	_templateRevision.FileMode = field.NewString(tableName, "file_mode")
	_templateRevision.User = field.NewString(tableName, "user")
	_templateRevision.UserGroup = field.NewString(tableName, "user_group")
	_templateRevision.Privilege = field.NewString(tableName, "privilege")
	_templateRevision.Signature = field.NewString(tableName, "signature")
	_templateRevision.ByteSize = field.NewUint64(tableName, "byte_size")
	_templateRevision.Md5 = field.NewString(tableName, "md5")
	_templateRevision.BizID = field.NewUint32(tableName, "biz_id")
	_templateRevision.TemplateSpaceID = field.NewUint32(tableName, "template_space_id")
	_templateRevision.TemplateID = field.NewUint32(tableName, "template_id")
	_templateRevision.Creator = field.NewString(tableName, "creator")
	_templateRevision.CreatedAt = field.NewTime(tableName, "created_at")

	_templateRevision.fillFieldMap()

	return _templateRevision
}

type templateRevision struct {
	templateRevisionDo templateRevisionDo

	ALL             field.Asterisk
	ID              field.Uint32
	RevisionName    field.String
	RevisionMemo    field.String
	Name            field.String
	Path            field.String
	FileType        field.String
	FileMode        field.String
	User            field.String
	UserGroup       field.String
	Privilege       field.String
	Signature       field.String
	ByteSize        field.Uint64
	Md5             field.String
	BizID           field.Uint32
	TemplateSpaceID field.Uint32
	TemplateID      field.Uint32
	Creator         field.String
	CreatedAt       field.Time

	fieldMap map[string]field.Expr
}

func (t templateRevision) Table(newTableName string) *templateRevision {
	t.templateRevisionDo.UseTable(newTableName)
	return t.updateTableName(newTableName)
}

func (t templateRevision) As(alias string) *templateRevision {
	t.templateRevisionDo.DO = *(t.templateRevisionDo.As(alias).(*gen.DO))
	return t.updateTableName(alias)
}

func (t *templateRevision) updateTableName(table string) *templateRevision {
	t.ALL = field.NewAsterisk(table)
	t.ID = field.NewUint32(table, "id")
	t.RevisionName = field.NewString(table, "revision_name")
	t.RevisionMemo = field.NewString(table, "revision_memo")
	t.Name = field.NewString(table, "name")
	t.Path = field.NewString(table, "path")
	t.FileType = field.NewString(table, "file_type")
	t.FileMode = field.NewString(table, "file_mode")
	t.User = field.NewString(table, "user")
	t.UserGroup = field.NewString(table, "user_group")
	t.Privilege = field.NewString(table, "privilege")
	t.Signature = field.NewString(table, "signature")
	t.ByteSize = field.NewUint64(table, "byte_size")
	t.Md5 = field.NewString(table, "md5")
	t.BizID = field.NewUint32(table, "biz_id")
	t.TemplateSpaceID = field.NewUint32(table, "template_space_id")
	t.TemplateID = field.NewUint32(table, "template_id")
	t.Creator = field.NewString(table, "creator")
	t.CreatedAt = field.NewTime(table, "created_at")

	t.fillFieldMap()

	return t
}

func (t *templateRevision) WithContext(ctx context.Context) ITemplateRevisionDo {
	return t.templateRevisionDo.WithContext(ctx)
}

func (t templateRevision) TableName() string { return t.templateRevisionDo.TableName() }

func (t templateRevision) Alias() string { return t.templateRevisionDo.Alias() }

func (t templateRevision) Columns(cols ...field.Expr) gen.Columns {
	return t.templateRevisionDo.Columns(cols...)
}

func (t *templateRevision) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := t.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (t *templateRevision) fillFieldMap() {
	t.fieldMap = make(map[string]field.Expr, 18)
	t.fieldMap["id"] = t.ID
	t.fieldMap["revision_name"] = t.RevisionName
	t.fieldMap["revision_memo"] = t.RevisionMemo
	t.fieldMap["name"] = t.Name
	t.fieldMap["path"] = t.Path
	t.fieldMap["file_type"] = t.FileType
	t.fieldMap["file_mode"] = t.FileMode
	t.fieldMap["user"] = t.User
	t.fieldMap["user_group"] = t.UserGroup
	t.fieldMap["privilege"] = t.Privilege
	t.fieldMap["signature"] = t.Signature
	t.fieldMap["byte_size"] = t.ByteSize
	t.fieldMap["md5"] = t.Md5
	t.fieldMap["biz_id"] = t.BizID
	t.fieldMap["template_space_id"] = t.TemplateSpaceID
	t.fieldMap["template_id"] = t.TemplateID
	t.fieldMap["creator"] = t.Creator
	t.fieldMap["created_at"] = t.CreatedAt
}

func (t templateRevision) clone(db *gorm.DB) templateRevision {
	t.templateRevisionDo.ReplaceConnPool(db.Statement.ConnPool)
	return t
}

func (t templateRevision) replaceDB(db *gorm.DB) templateRevision {
	t.templateRevisionDo.ReplaceDB(db)
	return t
}

type templateRevisionDo struct{ gen.DO }

type ITemplateRevisionDo interface {
	gen.SubQuery
	Debug() ITemplateRevisionDo
	WithContext(ctx context.Context) ITemplateRevisionDo
	WithResult(fc func(tx gen.Dao)) gen.ResultInfo
	ReplaceDB(db *gorm.DB)
	ReadDB() ITemplateRevisionDo
	WriteDB() ITemplateRevisionDo
	As(alias string) gen.Dao
	Session(config *gorm.Session) ITemplateRevisionDo
	Columns(cols ...field.Expr) gen.Columns
	Clauses(conds ...clause.Expression) ITemplateRevisionDo
	Not(conds ...gen.Condition) ITemplateRevisionDo
	Or(conds ...gen.Condition) ITemplateRevisionDo
	Select(conds ...field.Expr) ITemplateRevisionDo
	Where(conds ...gen.Condition) ITemplateRevisionDo
	Order(conds ...field.Expr) ITemplateRevisionDo
	Distinct(cols ...field.Expr) ITemplateRevisionDo
	Omit(cols ...field.Expr) ITemplateRevisionDo
	Join(table schema.Tabler, on ...field.Expr) ITemplateRevisionDo
	LeftJoin(table schema.Tabler, on ...field.Expr) ITemplateRevisionDo
	RightJoin(table schema.Tabler, on ...field.Expr) ITemplateRevisionDo
	Group(cols ...field.Expr) ITemplateRevisionDo
	Having(conds ...gen.Condition) ITemplateRevisionDo
	Limit(limit int) ITemplateRevisionDo
	Offset(offset int) ITemplateRevisionDo
	Count() (count int64, err error)
	Scopes(funcs ...func(gen.Dao) gen.Dao) ITemplateRevisionDo
	Unscoped() ITemplateRevisionDo
	Create(values ...*table.TemplateRevision) error
	CreateInBatches(values []*table.TemplateRevision, batchSize int) error
	Save(values ...*table.TemplateRevision) error
	First() (*table.TemplateRevision, error)
	Take() (*table.TemplateRevision, error)
	Last() (*table.TemplateRevision, error)
	Find() ([]*table.TemplateRevision, error)
	FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*table.TemplateRevision, err error)
	FindInBatches(result *[]*table.TemplateRevision, batchSize int, fc func(tx gen.Dao, batch int) error) error
	Pluck(column field.Expr, dest interface{}) error
	Delete(...*table.TemplateRevision) (info gen.ResultInfo, err error)
	Update(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	Updates(value interface{}) (info gen.ResultInfo, err error)
	UpdateColumn(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateColumnSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	UpdateColumns(value interface{}) (info gen.ResultInfo, err error)
	UpdateFrom(q gen.SubQuery) gen.Dao
	Attrs(attrs ...field.AssignExpr) ITemplateRevisionDo
	Assign(attrs ...field.AssignExpr) ITemplateRevisionDo
	Joins(fields ...field.RelationField) ITemplateRevisionDo
	Preload(fields ...field.RelationField) ITemplateRevisionDo
	FirstOrInit() (*table.TemplateRevision, error)
	FirstOrCreate() (*table.TemplateRevision, error)
	FindByPage(offset int, limit int) (result []*table.TemplateRevision, count int64, err error)
	ScanByPage(result interface{}, offset int, limit int) (count int64, err error)
	Scan(result interface{}) (err error)
	Returning(value interface{}, columns ...string) ITemplateRevisionDo
	UnderlyingDB() *gorm.DB
	schema.Tabler
}

func (t templateRevisionDo) Debug() ITemplateRevisionDo {
	return t.withDO(t.DO.Debug())
}

func (t templateRevisionDo) WithContext(ctx context.Context) ITemplateRevisionDo {
	return t.withDO(t.DO.WithContext(ctx))
}

func (t templateRevisionDo) ReadDB() ITemplateRevisionDo {
	return t.Clauses(dbresolver.Read)
}

func (t templateRevisionDo) WriteDB() ITemplateRevisionDo {
	return t.Clauses(dbresolver.Write)
}

func (t templateRevisionDo) Session(config *gorm.Session) ITemplateRevisionDo {
	return t.withDO(t.DO.Session(config))
}

func (t templateRevisionDo) Clauses(conds ...clause.Expression) ITemplateRevisionDo {
	return t.withDO(t.DO.Clauses(conds...))
}

func (t templateRevisionDo) Returning(value interface{}, columns ...string) ITemplateRevisionDo {
	return t.withDO(t.DO.Returning(value, columns...))
}

func (t templateRevisionDo) Not(conds ...gen.Condition) ITemplateRevisionDo {
	return t.withDO(t.DO.Not(conds...))
}

func (t templateRevisionDo) Or(conds ...gen.Condition) ITemplateRevisionDo {
	return t.withDO(t.DO.Or(conds...))
}

func (t templateRevisionDo) Select(conds ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.Select(conds...))
}

func (t templateRevisionDo) Where(conds ...gen.Condition) ITemplateRevisionDo {
	return t.withDO(t.DO.Where(conds...))
}

func (t templateRevisionDo) Order(conds ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.Order(conds...))
}

func (t templateRevisionDo) Distinct(cols ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.Distinct(cols...))
}

func (t templateRevisionDo) Omit(cols ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.Omit(cols...))
}

func (t templateRevisionDo) Join(table schema.Tabler, on ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.Join(table, on...))
}

func (t templateRevisionDo) LeftJoin(table schema.Tabler, on ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.LeftJoin(table, on...))
}

func (t templateRevisionDo) RightJoin(table schema.Tabler, on ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.RightJoin(table, on...))
}

func (t templateRevisionDo) Group(cols ...field.Expr) ITemplateRevisionDo {
	return t.withDO(t.DO.Group(cols...))
}

func (t templateRevisionDo) Having(conds ...gen.Condition) ITemplateRevisionDo {
	return t.withDO(t.DO.Having(conds...))
}

func (t templateRevisionDo) Limit(limit int) ITemplateRevisionDo {
	return t.withDO(t.DO.Limit(limit))
}

func (t templateRevisionDo) Offset(offset int) ITemplateRevisionDo {
	return t.withDO(t.DO.Offset(offset))
}

func (t templateRevisionDo) Scopes(funcs ...func(gen.Dao) gen.Dao) ITemplateRevisionDo {
	return t.withDO(t.DO.Scopes(funcs...))
}

func (t templateRevisionDo) Unscoped() ITemplateRevisionDo {
	return t.withDO(t.DO.Unscoped())
}

func (t templateRevisionDo) Create(values ...*table.TemplateRevision) error {
	if len(values) == 0 {
		return nil
	}
	return t.DO.Create(values)
}

func (t templateRevisionDo) CreateInBatches(values []*table.TemplateRevision, batchSize int) error {
	return t.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (t templateRevisionDo) Save(values ...*table.TemplateRevision) error {
	if len(values) == 0 {
		return nil
	}
	return t.DO.Save(values)
}

func (t templateRevisionDo) First() (*table.TemplateRevision, error) {
	if result, err := t.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateRevision), nil
	}
}

func (t templateRevisionDo) Take() (*table.TemplateRevision, error) {
	if result, err := t.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateRevision), nil
	}
}

func (t templateRevisionDo) Last() (*table.TemplateRevision, error) {
	if result, err := t.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateRevision), nil
	}
}

func (t templateRevisionDo) Find() ([]*table.TemplateRevision, error) {
	result, err := t.DO.Find()
	return result.([]*table.TemplateRevision), err
}

func (t templateRevisionDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*table.TemplateRevision, err error) {
	buf := make([]*table.TemplateRevision, 0, batchSize)
	err = t.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (t templateRevisionDo) FindInBatches(result *[]*table.TemplateRevision, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return t.DO.FindInBatches(result, batchSize, fc)
}

func (t templateRevisionDo) Attrs(attrs ...field.AssignExpr) ITemplateRevisionDo {
	return t.withDO(t.DO.Attrs(attrs...))
}

func (t templateRevisionDo) Assign(attrs ...field.AssignExpr) ITemplateRevisionDo {
	return t.withDO(t.DO.Assign(attrs...))
}

func (t templateRevisionDo) Joins(fields ...field.RelationField) ITemplateRevisionDo {
	for _, _f := range fields {
		t = *t.withDO(t.DO.Joins(_f))
	}
	return &t
}

func (t templateRevisionDo) Preload(fields ...field.RelationField) ITemplateRevisionDo {
	for _, _f := range fields {
		t = *t.withDO(t.DO.Preload(_f))
	}
	return &t
}

func (t templateRevisionDo) FirstOrInit() (*table.TemplateRevision, error) {
	if result, err := t.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateRevision), nil
	}
}

func (t templateRevisionDo) FirstOrCreate() (*table.TemplateRevision, error) {
	if result, err := t.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*table.TemplateRevision), nil
	}
}

func (t templateRevisionDo) FindByPage(offset int, limit int) (result []*table.TemplateRevision, count int64, err error) {
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

func (t templateRevisionDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = t.Count()
	if err != nil {
		return
	}

	err = t.Offset(offset).Limit(limit).Scan(result)
	return
}

func (t templateRevisionDo) Scan(result interface{}) (err error) {
	return t.DO.Scan(result)
}

func (t templateRevisionDo) Delete(models ...*table.TemplateRevision) (result gen.ResultInfo, err error) {
	return t.DO.Delete(models)
}

func (t *templateRevisionDo) withDO(do gen.Dao) *templateRevisionDo {
	t.DO = *do.(*gen.DO)
	return t
}
