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

	"github.com/TencentBlueKing/bk-bcs/bcs-services/bcs-bscp/pkg/dal/table"
)

func newUserPrivilege(db *gorm.DB, opts ...gen.DOOption) userPrivilege {
	_userPrivilege := userPrivilege{}

	_userPrivilege.userPrivilegeDo.UseDB(db, opts...)
	_userPrivilege.userPrivilegeDo.UseModel(&table.UserPrivilege{})

	tableName := _userPrivilege.userPrivilegeDo.TableName()
	_userPrivilege.ALL = field.NewAsterisk(tableName)
	_userPrivilege.ID = field.NewUint32(tableName, "id")
	_userPrivilege.User = field.NewString(tableName, "user")
	_userPrivilege.PrivilegeType = field.NewString(tableName, "privilege_type")
	_userPrivilege.ReadOnly = field.NewBool(tableName, "read_only")
	_userPrivilege.BizID = field.NewUint32(tableName, "biz_id")
	_userPrivilege.AppID = field.NewUint32(tableName, "app_id")
	_userPrivilege.Uid = field.NewUint32(tableName, "uid")
	_userPrivilege.TemplateSpaceID = field.NewUint32(tableName, "template_space_id")
	_userPrivilege.Creator = field.NewString(tableName, "creator")
	_userPrivilege.Reviser = field.NewString(tableName, "reviser")
	_userPrivilege.CreatedAt = field.NewTime(tableName, "created_at")
	_userPrivilege.UpdatedAt = field.NewTime(tableName, "updated_at")

	_userPrivilege.fillFieldMap()

	return _userPrivilege
}

type userPrivilege struct {
	userPrivilegeDo userPrivilegeDo

	ALL             field.Asterisk
	ID              field.Uint32
	User            field.String
	PrivilegeType   field.String
	ReadOnly        field.Bool
	BizID           field.Uint32
	AppID           field.Uint32
	Uid             field.Uint32
	TemplateSpaceID field.Uint32
	Creator         field.String
	Reviser         field.String
	CreatedAt       field.Time
	UpdatedAt       field.Time

	fieldMap map[string]field.Expr
}

func (u userPrivilege) Table(newTableName string) *userPrivilege {
	u.userPrivilegeDo.UseTable(newTableName)
	return u.updateTableName(newTableName)
}

func (u userPrivilege) As(alias string) *userPrivilege {
	u.userPrivilegeDo.DO = *(u.userPrivilegeDo.As(alias).(*gen.DO))
	return u.updateTableName(alias)
}

func (u *userPrivilege) updateTableName(table string) *userPrivilege {
	u.ALL = field.NewAsterisk(table)
	u.ID = field.NewUint32(table, "id")
	u.User = field.NewString(table, "user")
	u.PrivilegeType = field.NewString(table, "privilege_type")
	u.ReadOnly = field.NewBool(table, "read_only")
	u.BizID = field.NewUint32(table, "biz_id")
	u.AppID = field.NewUint32(table, "app_id")
	u.Uid = field.NewUint32(table, "uid")
	u.TemplateSpaceID = field.NewUint32(table, "template_space_id")
	u.Creator = field.NewString(table, "creator")
	u.Reviser = field.NewString(table, "reviser")
	u.CreatedAt = field.NewTime(table, "created_at")
	u.UpdatedAt = field.NewTime(table, "updated_at")

	u.fillFieldMap()

	return u
}

func (u *userPrivilege) WithContext(ctx context.Context) IUserPrivilegeDo {
	return u.userPrivilegeDo.WithContext(ctx)
}

func (u userPrivilege) TableName() string { return u.userPrivilegeDo.TableName() }

func (u userPrivilege) Alias() string { return u.userPrivilegeDo.Alias() }

func (u userPrivilege) Columns(cols ...field.Expr) gen.Columns {
	return u.userPrivilegeDo.Columns(cols...)
}

func (u *userPrivilege) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := u.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (u *userPrivilege) fillFieldMap() {
	u.fieldMap = make(map[string]field.Expr, 12)
	u.fieldMap["id"] = u.ID
	u.fieldMap["user"] = u.User
	u.fieldMap["privilege_type"] = u.PrivilegeType
	u.fieldMap["read_only"] = u.ReadOnly
	u.fieldMap["biz_id"] = u.BizID
	u.fieldMap["app_id"] = u.AppID
	u.fieldMap["uid"] = u.Uid
	u.fieldMap["template_space_id"] = u.TemplateSpaceID
	u.fieldMap["creator"] = u.Creator
	u.fieldMap["reviser"] = u.Reviser
	u.fieldMap["created_at"] = u.CreatedAt
	u.fieldMap["updated_at"] = u.UpdatedAt
}

func (u userPrivilege) clone(db *gorm.DB) userPrivilege {
	u.userPrivilegeDo.ReplaceConnPool(db.Statement.ConnPool)
	return u
}

func (u userPrivilege) replaceDB(db *gorm.DB) userPrivilege {
	u.userPrivilegeDo.ReplaceDB(db)
	return u
}

type userPrivilegeDo struct{ gen.DO }

type IUserPrivilegeDo interface {
	gen.SubQuery
	Debug() IUserPrivilegeDo
	WithContext(ctx context.Context) IUserPrivilegeDo
	WithResult(fc func(tx gen.Dao)) gen.ResultInfo
	ReplaceDB(db *gorm.DB)
	ReadDB() IUserPrivilegeDo
	WriteDB() IUserPrivilegeDo
	As(alias string) gen.Dao
	Session(config *gorm.Session) IUserPrivilegeDo
	Columns(cols ...field.Expr) gen.Columns
	Clauses(conds ...clause.Expression) IUserPrivilegeDo
	Not(conds ...gen.Condition) IUserPrivilegeDo
	Or(conds ...gen.Condition) IUserPrivilegeDo
	Select(conds ...field.Expr) IUserPrivilegeDo
	Where(conds ...gen.Condition) IUserPrivilegeDo
	Order(conds ...field.Expr) IUserPrivilegeDo
	Distinct(cols ...field.Expr) IUserPrivilegeDo
	Omit(cols ...field.Expr) IUserPrivilegeDo
	Join(table schema.Tabler, on ...field.Expr) IUserPrivilegeDo
	LeftJoin(table schema.Tabler, on ...field.Expr) IUserPrivilegeDo
	RightJoin(table schema.Tabler, on ...field.Expr) IUserPrivilegeDo
	Group(cols ...field.Expr) IUserPrivilegeDo
	Having(conds ...gen.Condition) IUserPrivilegeDo
	Limit(limit int) IUserPrivilegeDo
	Offset(offset int) IUserPrivilegeDo
	Count() (count int64, err error)
	Scopes(funcs ...func(gen.Dao) gen.Dao) IUserPrivilegeDo
	Unscoped() IUserPrivilegeDo
	Create(values ...*table.UserPrivilege) error
	CreateInBatches(values []*table.UserPrivilege, batchSize int) error
	Save(values ...*table.UserPrivilege) error
	First() (*table.UserPrivilege, error)
	Take() (*table.UserPrivilege, error)
	Last() (*table.UserPrivilege, error)
	Find() ([]*table.UserPrivilege, error)
	FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*table.UserPrivilege, err error)
	FindInBatches(result *[]*table.UserPrivilege, batchSize int, fc func(tx gen.Dao, batch int) error) error
	Pluck(column field.Expr, dest interface{}) error
	Delete(...*table.UserPrivilege) (info gen.ResultInfo, err error)
	Update(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	Updates(value interface{}) (info gen.ResultInfo, err error)
	UpdateColumn(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateColumnSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	UpdateColumns(value interface{}) (info gen.ResultInfo, err error)
	UpdateFrom(q gen.SubQuery) gen.Dao
	Attrs(attrs ...field.AssignExpr) IUserPrivilegeDo
	Assign(attrs ...field.AssignExpr) IUserPrivilegeDo
	Joins(fields ...field.RelationField) IUserPrivilegeDo
	Preload(fields ...field.RelationField) IUserPrivilegeDo
	FirstOrInit() (*table.UserPrivilege, error)
	FirstOrCreate() (*table.UserPrivilege, error)
	FindByPage(offset int, limit int) (result []*table.UserPrivilege, count int64, err error)
	ScanByPage(result interface{}, offset int, limit int) (count int64, err error)
	Scan(result interface{}) (err error)
	Returning(value interface{}, columns ...string) IUserPrivilegeDo
	UnderlyingDB() *gorm.DB
	schema.Tabler
}

func (u userPrivilegeDo) Debug() IUserPrivilegeDo {
	return u.withDO(u.DO.Debug())
}

func (u userPrivilegeDo) WithContext(ctx context.Context) IUserPrivilegeDo {
	return u.withDO(u.DO.WithContext(ctx))
}

func (u userPrivilegeDo) ReadDB() IUserPrivilegeDo {
	return u.Clauses(dbresolver.Read)
}

func (u userPrivilegeDo) WriteDB() IUserPrivilegeDo {
	return u.Clauses(dbresolver.Write)
}

func (u userPrivilegeDo) Session(config *gorm.Session) IUserPrivilegeDo {
	return u.withDO(u.DO.Session(config))
}

func (u userPrivilegeDo) Clauses(conds ...clause.Expression) IUserPrivilegeDo {
	return u.withDO(u.DO.Clauses(conds...))
}

func (u userPrivilegeDo) Returning(value interface{}, columns ...string) IUserPrivilegeDo {
	return u.withDO(u.DO.Returning(value, columns...))
}

func (u userPrivilegeDo) Not(conds ...gen.Condition) IUserPrivilegeDo {
	return u.withDO(u.DO.Not(conds...))
}

func (u userPrivilegeDo) Or(conds ...gen.Condition) IUserPrivilegeDo {
	return u.withDO(u.DO.Or(conds...))
}

func (u userPrivilegeDo) Select(conds ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.Select(conds...))
}

func (u userPrivilegeDo) Where(conds ...gen.Condition) IUserPrivilegeDo {
	return u.withDO(u.DO.Where(conds...))
}

func (u userPrivilegeDo) Order(conds ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.Order(conds...))
}

func (u userPrivilegeDo) Distinct(cols ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.Distinct(cols...))
}

func (u userPrivilegeDo) Omit(cols ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.Omit(cols...))
}

func (u userPrivilegeDo) Join(table schema.Tabler, on ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.Join(table, on...))
}

func (u userPrivilegeDo) LeftJoin(table schema.Tabler, on ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.LeftJoin(table, on...))
}

func (u userPrivilegeDo) RightJoin(table schema.Tabler, on ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.RightJoin(table, on...))
}

func (u userPrivilegeDo) Group(cols ...field.Expr) IUserPrivilegeDo {
	return u.withDO(u.DO.Group(cols...))
}

func (u userPrivilegeDo) Having(conds ...gen.Condition) IUserPrivilegeDo {
	return u.withDO(u.DO.Having(conds...))
}

func (u userPrivilegeDo) Limit(limit int) IUserPrivilegeDo {
	return u.withDO(u.DO.Limit(limit))
}

func (u userPrivilegeDo) Offset(offset int) IUserPrivilegeDo {
	return u.withDO(u.DO.Offset(offset))
}

func (u userPrivilegeDo) Scopes(funcs ...func(gen.Dao) gen.Dao) IUserPrivilegeDo {
	return u.withDO(u.DO.Scopes(funcs...))
}

func (u userPrivilegeDo) Unscoped() IUserPrivilegeDo {
	return u.withDO(u.DO.Unscoped())
}

func (u userPrivilegeDo) Create(values ...*table.UserPrivilege) error {
	if len(values) == 0 {
		return nil
	}
	return u.DO.Create(values)
}

func (u userPrivilegeDo) CreateInBatches(values []*table.UserPrivilege, batchSize int) error {
	return u.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (u userPrivilegeDo) Save(values ...*table.UserPrivilege) error {
	if len(values) == 0 {
		return nil
	}
	return u.DO.Save(values)
}

func (u userPrivilegeDo) First() (*table.UserPrivilege, error) {
	if result, err := u.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*table.UserPrivilege), nil
	}
}

func (u userPrivilegeDo) Take() (*table.UserPrivilege, error) {
	if result, err := u.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*table.UserPrivilege), nil
	}
}

func (u userPrivilegeDo) Last() (*table.UserPrivilege, error) {
	if result, err := u.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*table.UserPrivilege), nil
	}
}

func (u userPrivilegeDo) Find() ([]*table.UserPrivilege, error) {
	result, err := u.DO.Find()
	return result.([]*table.UserPrivilege), err
}

func (u userPrivilegeDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*table.UserPrivilege, err error) {
	buf := make([]*table.UserPrivilege, 0, batchSize)
	err = u.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (u userPrivilegeDo) FindInBatches(result *[]*table.UserPrivilege, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return u.DO.FindInBatches(result, batchSize, fc)
}

func (u userPrivilegeDo) Attrs(attrs ...field.AssignExpr) IUserPrivilegeDo {
	return u.withDO(u.DO.Attrs(attrs...))
}

func (u userPrivilegeDo) Assign(attrs ...field.AssignExpr) IUserPrivilegeDo {
	return u.withDO(u.DO.Assign(attrs...))
}

func (u userPrivilegeDo) Joins(fields ...field.RelationField) IUserPrivilegeDo {
	for _, _f := range fields {
		u = *u.withDO(u.DO.Joins(_f))
	}
	return &u
}

func (u userPrivilegeDo) Preload(fields ...field.RelationField) IUserPrivilegeDo {
	for _, _f := range fields {
		u = *u.withDO(u.DO.Preload(_f))
	}
	return &u
}

func (u userPrivilegeDo) FirstOrInit() (*table.UserPrivilege, error) {
	if result, err := u.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*table.UserPrivilege), nil
	}
}

func (u userPrivilegeDo) FirstOrCreate() (*table.UserPrivilege, error) {
	if result, err := u.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*table.UserPrivilege), nil
	}
}

func (u userPrivilegeDo) FindByPage(offset int, limit int) (result []*table.UserPrivilege, count int64, err error) {
	result, err = u.Offset(offset).Limit(limit).Find()
	if err != nil {
		return
	}

	if size := len(result); 0 < limit && 0 < size && size < limit {
		count = int64(size + offset)
		return
	}

	count, err = u.Offset(-1).Limit(-1).Count()
	return
}

func (u userPrivilegeDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = u.Count()
	if err != nil {
		return
	}

	err = u.Offset(offset).Limit(limit).Scan(result)
	return
}

func (u userPrivilegeDo) Scan(result interface{}) (err error) {
	return u.DO.Scan(result)
}

func (u userPrivilegeDo) Delete(models ...*table.UserPrivilege) (result gen.ResultInfo, err error) {
	return u.DO.Delete(models)
}

func (u *userPrivilegeDo) withDO(do gen.Dao) *userPrivilegeDo {
	u.DO = *do.(*gen.DO)
	return u
}
