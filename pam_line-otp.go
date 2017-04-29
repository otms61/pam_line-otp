package main

/*
#include <stdlib.h>
#include <security/pam_appl.h>
char* conversate(pam_handle_t *pamh, const char*);
*/
import "C"

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"log/syslog"
	"math/big"
	"os/exec"
	"runtime"
	"strings"
	"unsafe"
)

var conf Config

func infoLog(format string, args ...interface{}) {
	l, err := syslog.New(syslog.LOG_AUTHPRIV|syslog.LOG_INFO, "line-otp")
	if err != nil {
		return
	}
	l.Info(fmt.Sprintf(format, args...))
}

func errLog(format string, args ...interface{}) {
	l, err := syslog.New(syslog.LOG_AUTHPRIV|syslog.LOG_ERR, "line-otp")
	if err != nil {
		return
	}
	l.Err(fmt.Sprintf(format, args...))
}

type Config struct {
	DbPath          string
	LineAccessToken string
}

type User struct {
	AccountName string `gorm:"type:varchar(32);unique"`
	LineId      string `gorm:"type:varchar(40)"`
}

type Message struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Body struct {
	To       string    `json:"to"`
	Messages []Message `json:"messages"`
}

type AuthResult int

const (
	AuthError AuthResult = iota
	AuthSuccess
)

func initDb() (*gorm.DB, bool) {
	db, err := gorm.Open("sqlite3", conf.DbPath)
	if err != nil {
		infoLog("failed to connect database")
		return db, false
	}
	db.AutoMigrate(&User{})
	return db, true
}

func loadOptions(argv []string) bool {
	var ok bool
	m := make(map[string]string)
	for _, option := range argv {
		l := strings.Split(option, "=")
		k := strings.TrimSpace(l[0])
		v := strings.TrimSpace(l[1])
		m[k] = v
	}
	conf.DbPath, ok = m["DbPath"]
	if !ok {
		errLog("DbPath is not found!")
		return false
	}

	conf.LineAccessToken, ok = m["LineAccessToken"]
	if !ok {
		errLog("LineAccessToken is not found!")
		return false
	}
	return true

}

func findUser(username string) (User, bool) {
	var (
		count int
		user  User
	)

	db, ok := initDb()
	if !ok {
		return user, false
	}
	defer db.Close()

	db.Where("account_name = ?", username).First(&user).Count(&count)
	if count == 0 {
		infoLog("%s is not found", username)
		return user, false
	}
	return user, true

}

func pamAuthenticate(pamh *C.pam_handle_t, uid int, username string, argv []string) AuthResult {
	runtime.GOMAXPROCS(1)

	ok := loadOptions(argv)
	if !ok {
		errLog("Failed to load options")
		return AuthError
	}

	user, ok := findUser(username)
	if !ok {
		return AuthError
	}

	otp_n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		errLog("err %s\n", err)
		return AuthError
	}

	otp := fmt.Sprintf("%06d", otp_n)
	infoLog("%s's OTP is %s", username, otp)

	message := Message{
		Type: "text",
		Text: otp,
	}

	body := Body{
		To:       user.LineId,
		Messages: []Message{message},
	}

	b, err := json.Marshal(body)
	if err != nil {
		errLog("err %s\n", err)
		return AuthError
	}

	err = exec.Command("/bin/sh", "-c", `/usr/bin/curl -X POST \
  -H 'Content-Type:application/json' \
  -H 'Authorization: Bearer `+conf.LineAccessToken+`' \
  -d '`+string(b[:])+`' \
  https://api.line.me/v2/bot/message/push`).Run()

	if err != nil {
		infoLog("cmd err %s\n", err)
		return AuthError
	}

	prompt_message := C.CString(fmt.Sprintf("Line OTP: "))
	defer C.free(unsafe.Pointer(prompt_message))

	for i := 0; i < 3; i++ {
		user_input := C.GoString(C.conversate(pamh, prompt_message))
		if user_input == otp {
			infoLog("Line OTP verification successed")
			return AuthSuccess
		}
	}
	infoLog("Line OTP verification failed")
	return AuthError

}

func main() {}
