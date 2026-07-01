package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"secmgmt_go/internal/domain/entity"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func New(dsn string) (*gorm.DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&entity.OperationLog{},
		&entity.SystemSetting{},
	); err != nil {
		return err
	}
	if err := ensureDeviceCheckTables(db); err != nil {
		return err
	}
	return ensureSmartBridgeReconnectLogTable(db)
}

func ensureDeviceCheckTables(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS device_check_schedule (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			name VARCHAR(100) NOT NULL,
			enabled BOOL NOT NULL,
			frequency_per_day INTEGER NOT NULL,
			notify_enabled BOOL NOT NULL,
			push_config_ids_json TEXT,
			notify_mode VARCHAR(30) NOT NULL,
			last_run_at DATETIME,
			next_run_at DATETIME,
			last_success_at DATETIME,
			last_error TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			INDEX ix_device_check_schedule_enabled_next_run (enabled, next_run_at, id)
		)`,
		`CREATE TABLE IF NOT EXISTS device_check_run (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			schedule_id BIGINT UNSIGNED,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			status VARCHAR(20) NOT NULL,
			checked_total INTEGER NOT NULL,
			online_total INTEGER NOT NULL,
			offline_total INTEGER NOT NULL,
			disabled_total INTEGER NOT NULL,
			changed_total INTEGER NOT NULL,
			notified BOOL NOT NULL,
			error_message TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			INDEX ix_device_check_run_schedule_started (schedule_id, started_at, id),
			INDEX ix_device_check_run_status_started (status, started_at, id)
		)`,
		`CREATE TABLE IF NOT EXISTS device_check_push_log (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			schedule_id BIGINT UNSIGNED,
			run_id BIGINT UNSIGNED,
			push_config_id BIGINT UNSIGNED,
			status VARCHAR(30) NOT NULL,
			config_name VARCHAR(100),
			offline_count INTEGER NOT NULL,
			message VARCHAR(255),
			request_body TEXT,
			response_body TEXT,
			error_message TEXT,
			pushed_at DATETIME NOT NULL,
			PRIMARY KEY (id),
			INDEX ix_device_check_push_log_config_status_time (push_config_id, status, pushed_at, id),
			INDEX ix_device_check_push_log_run_id (run_id),
			INDEX ix_device_check_push_log_schedule_time (schedule_id, pushed_at, id)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureSmartBridgeReconnectLogTable(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS smart_bridge_reconnect_log (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			task_key VARCHAR(160) NOT NULL,
			cycle_key VARCHAR(120) NOT NULL,
			trigger_reason VARCHAR(60) NOT NULL,
			action VARCHAR(40) NOT NULL,
			status VARCHAR(30) NOT NULL,
			device_type VARCHAR(30) NOT NULL,
			device_id BIGINT UNSIGNED NOT NULL,
			session_key VARCHAR(120) NOT NULL,
			binding_ids_json TEXT,
			attempt INTEGER NOT NULL,
			max_attempts INTEGER NOT NULL,
			next_run_at DATETIME,
			detail TEXT,
			last_error TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			INDEX ix_smart_bridge_reconnect_log_created (created_at, id),
			INDEX ix_smart_bridge_reconnect_log_status_time (status, created_at, id),
			INDEX ix_smart_bridge_reconnect_log_device_time (device_type, device_id, created_at, id),
			INDEX ix_smart_bridge_reconnect_log_session_time (session_key, created_at, id),
			INDEX ix_smart_bridge_reconnect_log_task_time (task_key, created_at, id)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
