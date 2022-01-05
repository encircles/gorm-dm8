# gorm-dm8
达梦8 gorm驱动

```go
package main

import (
	"fmt"

	"github.com/encircles/gorm-dm8"
	"gorm.io/gorm"
)

func main() {

	// https://github.com/encircles/gorm-dm8
	dsn := "dm://SYSDBA:PASSWORD@127.0.0.1:5236?ignoreCase=false&appName=wisdom&statEnable=false"
	db, err := gorm.Open(dm8.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println(err)
	}

	type Result struct {
		ID       string `gorm:"ID"`
		USERNAME string `gorm:"USERNAME"`
		PASSWORD string `gorm:"PASSWORD"`
	}

	var result Result

	db.Raw("SELECT ID, USERNAME, PASSWORD FROM PERSON.PERSON WHERE ID = ?", "111").Scan(&result)

	fmt.Printf("%+v", result)
}

```