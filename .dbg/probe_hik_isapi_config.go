package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"secmgmt_go/internal/util"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

type recorder struct {
	ID                uint
	IP                string
	HTTPPort          int
	Username          string
	PasswordEncrypted string
}

func main() {
	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/secmgmt_db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rec, err := loadRecorder(db)
	if err != nil {
		log.Fatal(err)
	}
	password, err := util.ResolveDeviceSecret("change-me", rec.PasswordEncrypted)
	if err != nil {
		log.Fatal(err)
	}
	port := rec.HTTPPort
	if port <= 0 {
		port = 80
	}
	base := (&url.URL{Scheme: "http", Host: rec.IP + ":" + strconv.Itoa(port)}).String()
	client := &http.Client{Timeout: 8 * time.Second}
	paths := []string{
		"/ISAPI/Event/triggers",
		"/ISAPI/Event/triggers/VMD-1",
		"/ISAPI/Event/triggers/VMD-1/notifications",
		"/ISAPI/System/Video/inputs/channels/1/motionDetection",
		"/ISAPI/System/Video/inputs/channels/1/motionDetectionExt",
		"/ISAPI/Event/notification/httpHosts",
	}
	for _, path := range paths {
		body, status, err := getDigest(client, base+path, rec.Username, password)
		fmt.Printf("\n== %s status=%d err=%v ==\n", path, status, err)
		text := strings.TrimSpace(string(body))
		if len(text) > 1600 {
			text = text[:1600] + "..."
		}
		fmt.Println(text)
	}
}

func loadRecorder(db *sql.DB) (recorder, error) {
	var rec recorder
	err := db.QueryRow(`
SELECT r.id, r.ip, r.http_port, r.username, r.password_encrypted
FROM recorder_device r
JOIN smart_device_binding b ON b.source_type = 'recorder' AND b.source_id = r.id
JOIN smart_interface_provider p ON p.id = b.provider_id
WHERE p.provider_code = 'hikvision-isapi'
ORDER BY b.id ASC
LIMIT 1`).Scan(&rec.ID, &rec.IP, &rec.HTTPPort, &rec.Username, &rec.PasswordEncrypted)
	return rec, err
}

func getDigest(client *http.Client, targetURL, username, password string) ([]byte, int, error) {
	body, status, challenge, err := getOnce(client, targetURL, username, password, "")
	if err == nil && status >= 200 && status < 300 {
		return body, status, nil
	}
	if status != http.StatusUnauthorized || !strings.Contains(strings.ToLower(challenge), "digest") {
		return body, status, err
	}
	auth, err := buildDigestAuthorization(challenge, "GET", targetURL, username, password)
	if err != nil {
		return body, status, err
	}
	body, status, _, err = getOnce(client, targetURL, username, password, auth)
	return body, status, err
}

func getOnce(client *http.Client, targetURL, username, password, authorization string) ([]byte, int, string, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, "", err
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	} else {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	return body, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), readErr
}

func buildDigestAuthorization(challenge, method, targetURL, username, password string) (string, error) {
	values := parseDigestChallenge(strings.TrimSpace(strings.TrimPrefix(challenge, "Digest")))
	realm := values["realm"]
	nonce := values["nonce"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("invalid digest challenge")
	}
	req, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return "", err
	}
	qop := "auth"
	nc := "00000001"
	cnonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(method + ":" + req.URL.RequestURI())
	response := md5Hex(strings.Join([]string{ha1, nonce, nc, cnonce, qop, ha2}, ":"))
	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", qop=%s, nc=%s, cnonce="%s"`,
		username, realm, nonce, req.URL.RequestURI(), response, qop, nc, cnonce), nil
}

func parseDigestChallenge(value string) map[string]string {
	result := map[string]string{}
	for _, part := range strings.Split(value, ",") {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			result[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(raw), `"`)
		}
	}
	return result
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
