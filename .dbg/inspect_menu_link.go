package main

import (
	"fmt"
	"os"
	"path/filepath"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/database"
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cfg, err := config.Load(filepath.Clean(rootDir))
	if err != nil {
		panic(err)
	}
	db, err := database.New(cfg.MySQLDSN)
	if err != nil {
		panic(err)
	}

	type row struct {
		ID        int
		Code      string
		Name      string
		ParentID  *int
		RouteName *string
		Sort      int
		Status    string
	}
	var menu row
	if err := db.Raw("SELECT id, code, name, parent_id, route_name, sort, status FROM sys_menu WHERE code = ?", "safety-operation-logs").Scan(&menu).Error; err != nil {
		panic(err)
	}
	fmt.Printf("menu: %+v\n", menu)

	var roleLinkCount int64
	if err := db.Raw(`
SELECT COUNT(*)
FROM sys_role_menu rm
JOIN sys_role r ON r.id = rm.role_id
JOIN sys_menu m ON m.id = rm.menu_id
WHERE r.role_code = ? AND m.code = ?`,
			"admin", "safety-operation-logs",
		).Scan(&roleLinkCount).Error; err != nil {
		panic(err)
	}
	fmt.Printf("admin_role_menu_link: %d\n", roleLinkCount)
}
