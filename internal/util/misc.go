package util

import (
	"database/sql"
)

// This is horrible, what are you doing?
// Keeping read cursors open then doing more operations can be a little funky in some sqlite situations
// I want to make sure I'm always closing that read cursor when I can
// Then I started thinking about writing a generic read function
// And here we are
func AllRows1[T any](db *sql.DB, query string, result T, args ...any) ([]T, error) {
	results := make([]T, 0, 10)

	rows, err := db.Query(query, args...)
	if err != nil {
		return results, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&result)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}

	return results, err
}

// another return wanted? another function
// for the type safety, you know
// the other option is array-of-arrays with any type
// but then you're type wrangling at the call-site
// which also sucks
func AllRows2[T1 any, T2 any](db *sql.DB, query string, result1 T1, result2 T2, args ...any) ([]T1, []T2, error) {
	results1 := make([]T1, 0, 10)
	results2 := make([]T2, 0, 10)

	rows, err := db.Query(query, args...)
	if err != nil {
		return results1, results2, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&result1, &result2)
		if err != nil {
			return results1, results2, err
		}
		results1 = append(results1, result1)
		results2 = append(results2, result2)
	}

	return results1, results2, err
}
