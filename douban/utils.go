package douban

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"math/rand"
	"strings"
	"time"
)

func batchInsert(db *gorm.DB, objArr []interface{}) error {
	// If there is no data, nothing to do.
	if len(objArr) == 0 {
		return nil
	}

	mainObj := objArr[0]
	mainScope := db.NewScope(mainObj)
	mainFields := mainScope.Fields()
	quoted := make([]string, 0, len(mainFields))
	for i := range mainFields {
		if (mainFields[i].IsPrimaryKey && mainFields[i].IsBlank) || (mainFields[i].IsIgnored) {
			continue
		}
		quoted = append(quoted, mainScope.Quote(mainFields[i].DBName))
	}

	placeholdersArr := make([]string, 0, len(objArr))

	for _, obj := range objArr {
		scope := db.NewScope(obj)
		fields := scope.Fields()
		placeholders := make([]string, 0, len(fields))
		for i := range fields {
			if (fields[i].IsPrimaryKey && fields[i].IsBlank) || (fields[i].IsIgnored) {
				continue
			}
			placeholders = append(placeholders, scope.AddToVars(fields[i].Field.Interface()))
		}
		placeholdersStr := "(" + strings.Join(placeholders, ", ") + ")"
		placeholdersArr = append(placeholdersArr, placeholdersStr)
		// add real variables for the replacement of placeholders' '?' letter later.
		mainScope.SQLVars = append(mainScope.SQLVars, scope.SQLVars...)
	}

	mainScope.Raw(fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES %s",
		mainScope.QuotedTableName(),
		strings.Join(quoted, ", "),
		strings.Join(placeholdersArr, ", "),
	))

	if res, err := mainScope.SQLDB().Exec(mainScope.SQL, mainScope.SQLVars...); err != nil {
		return err
	} else {
		rowsAffected, _ := res.RowsAffected()
		Logger.Infof("batchInsert size:%d ,rowsAffected: %d", len(objArr), rowsAffected)
	}
	return nil
}

func isEmpty(str string) bool {
	if str != "" && len(str) > 0 {
		return false
	}
	return true
}

func friendlyToDouban() {
	r := rand.Intn(3) + 5 //random integer 5~8
	Logger.Infof("sleep for :%d second...", r)
	time.Sleep(time.Duration(r) * time.Second)
}

func trimString(str string) string {
	return strings.Replace(str, " ", "", -1)
}

func trimNewline(str string) string {
	return strings.Replace(str, "\n", "", -1)
}
