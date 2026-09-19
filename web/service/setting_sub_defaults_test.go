package service

import (
	"path/filepath"
	"testing"

	"x-ui/database"
	"x-ui/database/model"
)

func TestApplySubSettingDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	if err := database.InitDB(filepath.Join(tmpDir, "x-ui.db")); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	// Windows 下必须先关闭 sqlite 连接，否则测试结束时临时目录删不掉
	defer func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			sqlDB.Close()
		}
	}()

	db := database.GetDB()

	// 模拟一个「还是旧默认值」的老面板数据库
	for _, row := range []model.Setting{
		{Key: "subEnable", Value: "false"},
		{Key: "subPort", Value: "13788"},
		{Key: "subPath", Value: "/sub/"},
		{Key: "subJsonPath", Value: "/json/"},
		{Key: "subDomain", Value: "example.com"}, // 用户自己设置过，必须保留
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("写入设置失败: %v", err)
		}
	}

	s := &SettingService{}
	if err := s.ApplySubSettingDefaults(); err != nil {
		t.Fatalf("升级失败: %v", err)
	}

	get := func(key string) string {
		setting := &model.Setting{}
		if err := db.Where("key = ?", key).First(setting).Error; err != nil {
			t.Fatalf("读取 %s 失败: %v", key, err)
		}
		return setting.Value
	}

	for key, want := range map[string]string{
		"subEnable":   "true",
		"subPort":     "58888",
		"subPath":     "/yfzgsub/",
		"subJsonPath": "/yfzgjson/",
		"subDomain":   "example.com",
	} {
		if got := get(key); got != want {
			t.Fatalf("%s = %q, 期望 %q", key, got, want)
		}
	}

	// 幂等：再跑一次不应报错
	if err := s.ApplySubSettingDefaults(); err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}

	// 用户后来自己改了端口，即使标记被清掉也不能被覆盖
	if err := s.saveSetting("subPort", "12345"); err != nil {
		t.Fatalf("写入端口失败: %v", err)
	}
	db.Where("key = ?", subSettingMigratedKey).Delete(&model.Setting{})
	if err := s.ApplySubSettingDefaults(); err != nil {
		t.Fatalf("再次升级失败: %v", err)
	}
	if got := get("subPort"); got != "12345" {
		t.Fatalf("用户自定义端口被覆盖: %q", got)
	}
}

func TestSubCertFallsBackToPanelCert(t *testing.T) {
	tmpDir := t.TempDir()
	if err := database.InitDB(filepath.Join(tmpDir, "x-ui.db")); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	defer func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			sqlDB.Close()
		}
	}()

	s := &SettingService{}

	// 只配置了面板证书，订阅证书留空 → 订阅应自动沿用面板证书
	if err := s.SetCertFile("/etc/ssl/panel/cert.pem"); err != nil {
		t.Fatalf("写入面板证书失败: %v", err)
	}
	if err := s.SetKeyFile("/etc/ssl/panel/key.pem"); err != nil {
		t.Fatalf("写入面板私钥失败: %v", err)
	}
	if got, err := s.GetSubCertFile(); err != nil || got != "/etc/ssl/panel/cert.pem" {
		t.Fatalf("订阅证书 = %q (err=%v), 期望沿用面板证书", got, err)
	}
	if got, err := s.GetSubKeyFile(); err != nil || got != "/etc/ssl/panel/key.pem" {
		t.Fatalf("订阅私钥 = %q (err=%v), 期望沿用面板私钥", got, err)
	}

	// 订阅单独配置了证书 → 使用自己的
	if err := s.setString("subCertFile", "/etc/ssl/sub/cert.pem"); err != nil {
		t.Fatalf("写入订阅证书失败: %v", err)
	}
	if err := s.setString("subKeyFile", "/etc/ssl/sub/key.pem"); err != nil {
		t.Fatalf("写入订阅私钥失败: %v", err)
	}
	if got, _ := s.GetSubCertFile(); got != "/etc/ssl/sub/cert.pem" {
		t.Fatalf("订阅证书 = %q, 期望使用订阅自己的证书", got)
	}
	if got, _ := s.GetSubKeyFile(); got != "/etc/ssl/sub/key.pem" {
		t.Fatalf("订阅私钥 = %q, 期望使用订阅自己的私钥", got)
	}
}
