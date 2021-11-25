package dm8

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestT1(t *testing.T) {
	// str := `"TEST.WISDOM".jwt_blacklist`
	str := `jwt_blacklist`

	s := splitTableStr(str)

	t.Log(s)
}

func splitTableStr(str string) []string {
	// str format = `"XXX.XXX".XXX`
	match, _ := regexp.MatchString("\"(.+)[.](.+)\"[.](.+)", str)
	fmt.Println(match) // true

	s := strings.Split(strings.TrimLeft(str, "\""), "\".")
	return s
}
