package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"          // PostgreSQL driver
	_ "github.com/sijms/go-ora/v2" // Oracle driver
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

		http.Redirect(w, r, "/sqltool/query", http.StatusSeeOther)
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

		columns, err := rows.Columns()
		if err != nil {
			qr.Error = "获取列名失败: " + err.Error()
			tmpl.ExecuteTemplate(w, "query.html", qr)
			return
		}
		qr.Columns = columns

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		qr.Data = []map[string]interface{}{}

		for rows.Next() {
			rowData := make(map[string]interface{})
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				qr.Error = "扫描行数据失败: " + err.Error()
				tmpl.ExecuteTemplate(w, "query.html", qr)
				return
			}

			for i, col := range columns {
				val := values[i]

				b, ok := val.([]byte)
				var strVal interface{}
				if !ok {
					strVal = val
				} else if len(b) == 0 {
					strVal = "NULL"
				} else {
					strVal = string(b)
				}

				rowData[col] = strVal
			}

			qr.Data = append(qr.Data, rowData)
		}

		if err := rows.Err(); err != nil {
			qr.Error = "遍历行数据失败: " + err.Error()
			tmpl.ExecuteTemplate(w, "query.html", qr)
			return
		}

		if len(qr.Data) == 0 {
			qr.NoData = true
		} else {
			qr.Message = "查询成功."
		}

		// Debugging: Print the final QueryResult structure to verify it's correct
		log.Printf("Final Query Result:\nColumns: %v\nData: %v\n", qr.Columns, qr.Data)

		// Ensure template executes correctly
		err = tmpl.ExecuteTemplate(w, "query.html", qr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// Ensure template executes correctly for GET requests as well
	err := tmpl.ExecuteTemplate(w, "query.html", qr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
func main() {
	r := mux.NewRouter()
	r.HandleFunc("/sqltool", indexHandler).Methods("GET", "POST")
	r.HandleFunc("/sqltool/query", queryHandler).Methods("GET", "POST")

	// 添加静态文件服务
	fs := http.FileServer(http.Dir("./static"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	log.Println("Server started at http://localhost:7699")
	log.Fatal(http.ListenAndServe(":7699", r))
}
