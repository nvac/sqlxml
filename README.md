## sqlxml

<hr/>

An easy-to-use extension for [sqlx](https://github.com/jmoiron/sqlx) ，base on xml files and named query/exec

<p style="color: orangered">this repo is under development, please do not use it in production.</p>

### install

``go get github.com/nvac/sqlxml``

### Usage

#### 1. set database config in xml file

* `name`: needs to be unique in same environment
* `env`: custom string，runtime environment
* `driver`: database driver
* `dsn`: data source name
* `maxIdleConns`: sets the maximum number of connections in the idle connection pool. if default values is required,
  remove the attr
* `maxOpenConns`: sets the maximum number of open connections to the database. if default values is required, remove the
  attr
* `connMaxLifetimeSeconds`: sets the maximum amount of time a connection may be reused. . if default values is required,
  remove the attr
* `connMaxIdleTimeSeconds`: sets the maximum amount of time a connection may be idle. if default values is required,
  remove the attr

````xml
<?xml version="1.0" encoding="utf-8" ?>

<databases>
    <database name="ReadDb"
              env="dev"
              driver="mysql"
              dsn="user:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&amp;parseTime=True"
              maxIdleConns="5"
              maxOpenConns="10"
              connMaxLifetimeSeconds="30"
              connMaxIdleTimeSeconds="30"
    />
    
    <database name="WriteDb"
              env="dev"
              driver="mysql"
              dsn="user:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&amp;parseTime=True"
              maxIdleConns="5"
              maxOpenConns="10"
              connMaxLifetimeSeconds="30"
              connMaxIdleTimeSeconds="30"
    />
</databases>
````

#### 2. write sql script in xml file

* name: needs to be unique
* database: using the above configured database
* content: ensure in CDATA

````xml
<?xml version="1.0" encoding="utf-8" ?>

<scripts>
    <script name="GetUser">
        <![CDATA[
            SELECT username, password
            FROM `user`
            WHERE username = :username
        ]]>
    </script>

    <script name="ListUser">
        <![CDATA[
            SELECT username, password
            FROM `user`
            LIMIT 10 OFFSET 0
        ]]>
    </script>
    
    <script name="AddUser">
        <![CDATA[
            INSERT INTO user (username, password)
            VALUES (:username, :password)
        ]]>
    </script>
</scripts>
````

3. inti & use sqlxml

````go
package main

import (
  "context"

  "github.com/nvac/sqlxml"
)

type User struct {
  Username string `db:"username"`
  Password string `db:"password"`
}

func main() {
  opts := &sqlxml.Options{
    DatabaseFile:     "config/databases.xml",
    ScriptsGlobFiles: "config/scripts/*.xml",
    Env:              "dev",
    DsnDecryptFunc: func(source string) string {
      return source
    },
  }

  client := sqlxml.NewClient(opts)
  if client.Error() != nil {
    panic(client.Error())
  }

  readDb := client.Database("ReadDb")
  if readDb.Error() != nil {
    panic(readDb.Error())
  }

  ctx := context.TODO()

  var user User
  if err := readDb.QueryRow(ctx, "GetUser", &user, map[string]interface{}{
    "username": "lisa",
  }); err != nil {
    panic(err)
  }

  var users []User
  if err := readDb.QueryRows(ctx, "ListUser", &users, map[string]interface{}{}); err != nil {
    panic(err)
  }

  writeDb := client.Database("ReadDb")
  if writeDb.Error() != nil {
    panic(writeDb.Error())
  }

  if _, err := writeDb.Exec(ctx, "AddUser", map[string]interface{}{
    "username": "root",
    "password": "123456",
  }); err != nil {
    panic(err)
  }
}
````

### License

[MIT](LICENSE) © nvac