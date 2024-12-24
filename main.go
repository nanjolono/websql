package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"   // PostgreSQL driver
	_ "gopkg.in/goracle.v2" // Oracle driver
	"html/template"
)

var db *sql.DB
var tmpl = template.Must(template.ParseGlob("templates/*.html"))

type QueryResult struct {
	Error   string
	Message string
	Data    []map[string]interface{}
	Columns []string
	NoData  bool
}

func initDB(driver, dsn string) error {
	var err error
	db, err = sql.Open(driver, dsn)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return err
	}
	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		ip := r.FormValue("ip")
		user := r.FormValue("user")
		password := r.FormValue("password")
		dbName := r.FormValue("dbname")
		driver := r.FormValue("driver")

		var dsn string
		switch driver {
		case "mysql":
			dsn = fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", user, password, ip, dbName)
		case "postgres":
			dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", ip, user, password, dbName)
		case "oracle":
			dsn = fmt.Sprintf("%s/%s@%s/%s", user, password, ip, dbName)
		default:
			http.Error(w, "未知的数据库驱动.", http.StatusBadRequest)
			return
		}

		err := initDB(driver, dsn)
		if err != nil {
			http.Error(w, "无法连接到数据库，请检查信息后重试.", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/query", http.StatusSeeOther)
		return
	}

	tmpl.ExecuteTemplate(w, "index.html", nil)
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	qr := QueryResult{}

	if r.Method == "POST" {
		query := r.FormValue("query")
		rows, err := db.Query(query)
		if err != nil {
			qr.Error = "查询失败: " + err.Error()
			tmpl.ExecuteTemplate(w, "query.html", qr)
			return
		}
		defer rows.Close()

		columns, _ := rows.Columns()
		qr.Columns = columns

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		qr.Data = []map[string]interface{}{}
		for rows.Next() {
			for i := range columns {
				valuePtrs[i] = &values[i]
			}
			rows.Scan(valuePtrs...)
			entry := make(map[string]interface{})
			for i, col := range columns {
				var val interface{}
				switch v := values[i].(type) {
				case []byte:
					val = string(v)
				default:
					val = v
				}
				entry[col] = val
			}
			qr.Data = append(qr.Data, entry)
		}

		if len(qr.Data) == 0 {
			qr.NoData = true
		} else {
			qr.Message = "查询成功."
		}

		tmpl.ExecuteTemplate(w, "query.html", qr)
		return
	}

	tmpl.ExecuteTemplate(w, "query.html", qr)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler).Methods("GET", "POST")
	r.HandleFunc("/query", queryHandler).Methods("GET", "POST")

	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
