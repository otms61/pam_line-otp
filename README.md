```sh
$ go build -buildmode=c-shared -o pam_line-otp.so
```

```
auth    required    /home/saso/pam_line-otp/pam_line-otp.so DbPath=/path/to/pam_line-otp.db LineAccessToken=XXXXXX
```

```sh
$ sudo sqlite3 /etc/pam_line-otp.db 'CREATE TABLE "users" ("account_name" varchar(32) UNIQUE,"line_id" varchar(40) )'
$ sudo sqlite3 /etc/pam_line-otp.db 'INSERT INTO "users" VALUES ("test", $LINE_UID);'
```
