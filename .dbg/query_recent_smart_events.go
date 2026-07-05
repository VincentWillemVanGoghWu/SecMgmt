package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/secmgmt_db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query(db, "smart_event", `
SELECT e.id, COALESCE(p.provider_code, ''), e.event_code, e.event_type, e.event_time, e.created_at, COALESCE(e.image_url, '')
FROM smart_event e
LEFT JOIN smart_interface_provider p ON p.id = e.provider_id
ORDER BY e.id DESC
LIMIT 10`)

	query(db, "smart_raw_event", `
SELECT r.id, COALESCE(p.provider_code, ''), r.source_event_id, r.event_no, r.event_time, r.created_at, r.parse_status, COALESCE(r.raw_payload_json, '')
FROM smart_raw_event r
LEFT JOIN smart_interface_provider p ON p.id = r.provider_id
ORDER BY r.id DESC
LIMIT 10`)

	query(db, "alarm_record", `
SELECT a.id, a.alarm_no, a.alarm_type, a.alarm_time, a.created_at, COALESCE(p.provider_code, ''), COALESCE(a.image_url, '')
FROM alarm_record a
LEFT JOIN smart_event e ON e.id = a.smart_event_id
LEFT JOIN smart_interface_provider p ON p.id = e.provider_id
ORDER BY a.id DESC
LIMIT 10`)
}

func query(db *sql.DB, title, statement string) {
	fmt.Println("==", title, "==")
	rows, err := db.Query(statement)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
	}
	values := make([]any, len(cols))
	scan := make([]any, len(cols))
	for i := range values {
		scan[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(scan...); err != nil {
			log.Fatal(err)
		}
		for i, col := range cols {
			if i > 0 {
				fmt.Print(" | ")
			}
			fmt.Print(col, "=")
			switch v := values[i].(type) {
			case nil:
				fmt.Print("")
			case []byte:
				text := string(v)
				if len(text) > 240 {
					text = text[:240] + "..."
				}
				fmt.Print(text)
			case time.Time:
				fmt.Print(v.Format("2006-01-02 15:04:05"))
			default:
				fmt.Print(v)
			}
		}
		fmt.Println()
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
