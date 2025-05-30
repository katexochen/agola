package gen

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/huandu/xstrings"

	"agola.io/agola/internal/sqlg"
	"agola.io/agola/internal/sqlg/sql"
)

type DDLGenericData struct {
	Version   uint
	Sequences []DDLDataTableSequence
	Tables    []DDLGenericDataTableInfo
	Data      map[sql.Type]DDLData
}

type DDLGenericDataTableInfo struct {
	Name    string
	Columns []DDLGenericDataColInfo
}

type DDLGenericDataColInfo struct {
	Name     string
	Type     string
	JSON     bool
	Nullable bool
}

type DDLData struct {
	Version   uint
	DBType    string
	TableDefs []DDLDataTable
	IndexDefs []string
}

type DDLDataTable struct {
	Table          string
	ColumnDefs     []string
	ConstraintDefs []string
	DDL            string
}

type DDLDataTableSequence struct {
	Name      string
	TableName string
	ColName   string
}

func genDDLGenericData(gd *genData) DDLGenericData {
	objectsInfo := []sqlg.ObjectInfo{}
	for _, oi := range gd.ObjectsInfo {
		oi.Fields = append(objectMetaFields(), oi.Fields...)

		objectsInfo = append(objectsInfo, oi)
	}

	objectsInfo = sqlg.PopulateObjectsInfo(objectsInfo, "")

	data := DDLGenericData{
		Version: gd.Version,
		Data:    make(map[sql.Type]DDLData),
	}

	for _, oi := range objectsInfo {
		tableInfo := DDLGenericDataTableInfo{
			Name:    oi.Table,
			Columns: []DDLGenericDataColInfo{},
		}

		for _, of := range oi.Fields {
			colName := of.ColName

			if of.Sequence {
				sequenceName := fmt.Sprintf("%s_%s_seq", oi.Table, colName)
				data.Sequences = append(data.Sequences, DDLDataTableSequence{
					Name:      sequenceName,
					TableName: oi.Table,
					ColName:   colName,
				})
			}

			tableInfo.Columns = append(tableInfo.Columns, DDLGenericDataColInfo{
				Name:     colName,
				Type:     of.BaseType,
				JSON:     of.JSON,
				Nullable: of.Nullable,
			})
		}

		data.Tables = append(data.Tables, tableInfo)
	}

	return data
}

func genDDLData(gd *genData, dbType sql.Type) DDLData {
	objectsInfo := []sqlg.ObjectInfo{}
	for _, oi := range gd.ObjectsInfo {
		oi.Fields = append(objectMetaFields(), oi.Fields...)

		objectsInfo = append(objectsInfo, oi)
	}

	objectsInfo = sqlg.PopulateObjectsInfo(objectsInfo, dbType)

	data := DDLData{
		Version: gd.Version,
		DBType:  xstrings.FirstRuneToUpper(string(dbType)),
	}

	for _, oi := range objectsInfo {
		tableDef := DDLDataTable{Table: oi.Table}

		for _, of := range oi.Fields {
			colName := of.ColName
			colDef := fmt.Sprintf("%s %s", colName, of.SQLType)
			if !of.Nullable {
				colDef += " NOT NULL"
			}
			if of.Unique {
				colDef += " UNIQUE"
			}

			tableDef.ColumnDefs = append(tableDef.ColumnDefs, colDef)
		}

		tableDef.ConstraintDefs = oi.Constraints

		tableDef.DDL = fmt.Sprintf(`create table if not exists %s (%s, PRIMARY KEY (id)`, tableDef.Table, strings.Join(tableDef.ColumnDefs, ", "))
		if len(tableDef.ConstraintDefs) > 0 {
			tableDef.DDL += ", "
			tableDef.DDL += strings.Join(tableDef.ConstraintDefs, ", ")
		}
		tableDef.DDL += ")"

		data.TableDefs = append(data.TableDefs, tableDef)

		data.IndexDefs = append(data.IndexDefs, oi.Indexes...)
	}

	return data
}

func genDDL(gd *genData) {
	data := genDDLGenericData(gd)
	for _, dbType := range []sql.Type{sql.Postgres, sql.Sqlite3} {
		data.Data[dbType] = genDDLData(gd, dbType)
	}

	f, err := os.Create("ddl.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := ddlTemplate.Execute(f, data); err != nil {
		panic(err)
	}
}

var ddlTemplate = template.Must(template.New("").Funcs(sprig.TxtFuncMap()).Funcs(funcs).Parse(`// Code generated by go generate; DO NOT EDIT.
package db

import (
	"agola.io/agola/internal/sqlg"
)

{{- range $dbType, $data := .Data }}
var DDL{{ $data.DBType }} = []string{
{{- range $tableDef := $data.TableDefs }}
	"{{ $tableDef.DDL }}",
{{- end }}

	// indexes

{{- range $index := .IndexDefs }}
	"{{ $index }}",
{{- end }}
}
{{- end }}

var Sequences = []sqlg.Sequence {
{{- range $sequence := .Sequences }}
	{
		Name:   "{{ $sequence.Name }}",
		Table:  "{{ $sequence.TableName }}",
		Column: "{{ $sequence.ColName }}",
	},
{{- end }}
}
`))
