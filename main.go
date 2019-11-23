package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jimsmart/schema"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/jinzhu/inflection"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/serenize/snaker"
	"github.com/smallnest/gen/dbmeta"
	gtmpl "github.com/smallnest/gen/template"
)

func main() {

	driverName := flag.String("driver", "", "mysql, postgres")
	host := flag.String("host", "", "127.0.0.1")
	port := flag.String("port", "", "5432")
	user := flag.String("user", "", "user_name")
	dbname := flag.String("dbname", "", "database name")

	packageName := flag.String("package", "", "package name")

	jsonAnnotation := flag.Bool("json", false, "json annotate")
	verbose := flag.Bool("v", false, "verbose")
	gormAnnotation := flag.Bool("gorm", true, "gorm annotate")
	gureguTypes := flag.Bool("guregu", false, "use guregu")
	rest := flag.Bool("rest", false, "")

	flag.Parse()

	dataSourceName := fmt.Sprintf("sslmode=disable host=%v port=%v user=%v dbname=%v", *host, *port, *user, *dbname)
	fmt.Println(*driverName)
	fmt.Println(dataSourceName)
	db, err := sql.Open(*driverName, dataSourceName)
	if err != nil {
		fmt.Println("Error in open database: " + err.Error())
		return
	}
	defer db.Close()

	// parse or read tables
	var tables []string
	tables, err = schema.TableNames(db)
	if err != nil {
		fmt.Println(err)
		return
	}
	// if packageName is not set we need to default it
	if packageName == nil || *packageName == "" {
		*packageName = "generated"
	}
	os.Mkdir("model", 0777)

	apiName := "api"
	if *rest {
		os.Mkdir(apiName, 0777)
	}

	t, err := getTemplate(gtmpl.ModelTmpl)
	if err != nil {
		fmt.Println("Error in loading model template: " + err.Error())
		return
	}

	ct, err := getTemplate(gtmpl.ControllerTmpl)
	if err != nil {
		fmt.Println("Error in loading controller template: " + err.Error())
		return
	}

	var structNames []string

	// generate go files for each table
	for _, tableName := range tables {
		structName := dbmeta.FmtFieldName(tableName)
		structName = inflection.Singular(structName)
		structNames = append(structNames, structName)

		modelInfo := dbmeta.GenerateStruct(db, *driverName, tableName, structName, "model", *jsonAnnotation, *gormAnnotation, *gureguTypes)

		if *verbose {
			fmt.Println(modelInfo)
		}

		var buf bytes.Buffer
		err = t.Execute(&buf, modelInfo)
		if err != nil {
			fmt.Println("Error in rendering model: " + err.Error())
			return
		}
		data, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Println("Error in formating source: " + err.Error())
			return
		}
		ioutil.WriteFile(filepath.Join("model", inflection.Singular(tableName)+".go"), data, 0777)

		if *rest {
			//write api
			buf.Reset()
			err = ct.Execute(&buf, map[string]string{"PackageName": *packageName + "/model", "StructName": structName})
			if err != nil {
				fmt.Println("Error in rendering controller: " + err.Error())
				return
			}
			data, err = format.Source(buf.Bytes())
			if err != nil {
				fmt.Println("Error in formating source: " + err.Error())
				return
			}
			ioutil.WriteFile(filepath.Join(apiName, inflection.Singular(tableName)+".go"), data, 0777)
		}
	}

	if *rest {
		rt, err := getTemplate(gtmpl.RouterTmpl)
		if err != nil {
			fmt.Println("Error in lading router template")
			return
		}
		var buf bytes.Buffer
		err = rt.Execute(&buf, structNames)
		if err != nil {
			fmt.Println("Error in rendering router: " + err.Error())
			return
		}
		data, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Println("Error in formating source: " + err.Error())
			return
		}
		ioutil.WriteFile(filepath.Join(apiName, "router.go"), data, 0777)
	}
}

func getTemplate(t string) (*template.Template, error) {
	var funcMap = template.FuncMap{
		"pluralize":        inflection.Plural,
		"title":            strings.Title,
		"toLower":          strings.ToLower,
		"toLowerCamelCase": camelToLowerCamel,
		"toSnakeCase":      snaker.CamelToSnake,
	}

	tmpl, err := template.New("model").Funcs(funcMap).Parse(t)

	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func camelToLowerCamel(s string) string {
	ss := strings.Split(s, "")
	ss[0] = strings.ToLower(ss[0])

	return strings.Join(ss, "")
}
