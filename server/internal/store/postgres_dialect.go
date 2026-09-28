package store

import (
	"fmt"
	"regexp"
	"strings"
)

var dateOffsetPattern = regexp.MustCompile(`(?i)julianday\(\?,\s*'-10 minutes'\)`)
var julianDayPattern = regexp.MustCompile(`(?i)julianday\(([^()]+)\)`)

// Legacy flags are stored as 0/1 SMALLINT in PostgreSQL, as in SQLite.
func postgresArgs(args []any) []any {
	converted := append([]any(nil), args...)
	for i, value := range args {
		if flag, ok := value.(bool); ok {
			if flag {
				converted[i] = int16(1)
			} else {
				converted[i] = int16(0)
			}
		}
	}
	return converted
}

func rewritePostgresSQL(query string) string {
	trimmed := strings.TrimSpace(query)
	insertIgnore := strings.HasPrefix(strings.ToUpper(trimmed), "INSERT OR IGNORE INTO ")
	if insertIgnore {
		query = strings.Replace(query, "INSERT OR IGNORE INTO ", "INSERT INTO ", 1)
	}
	query = strings.ReplaceAll(query, "username = ? COLLATE NOCASE", "lower(username) = lower(?)")
	query = strings.ReplaceAll(query, "email = ? COLLATE NOCASE", "lower(email) = lower(?)")
	query = strings.ReplaceAll(query, "ORDER BY username COLLATE NOCASE", "ORDER BY lower(username)")
	query = dateOffsetPattern.ReplaceAllString(query, `(?::timestamptz - interval '10 minutes')`)
	query = julianDayPattern.ReplaceAllString(query, `($1::timestamptz)`)
	query = strings.ReplaceAll(query, "next_attempt_at <= ?", "NULLIF(next_attempt_at, '')::timestamptz <= ?::timestamptz")
	if insertIgnore {
		query = strings.TrimSuffix(strings.TrimSpace(query), ";") + " ON CONFLICT DO NOTHING"
	}
	return rebindPostgres(query)
}

// rebindPostgres replaces only SQL parameter markers. Quoted SQL, comments and
// dollar-quoted strings are copied without interpreting their contents.
func rebindPostgres(query string) string {
	var out strings.Builder
	out.Grow(len(query) + 16)
	parameter := 0
	for i := 0; i < len(query); {
		switch {
		case query[i] == '\'' || query[i] == '"':
			quote := query[i]
			start := i
			i++
			for i < len(query) {
				if query[i] == quote {
					i++
					if i < len(query) && query[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
			out.WriteString(query[start:i])
		case strings.HasPrefix(query[i:], "--"):
			end := strings.IndexByte(query[i:], '\n')
			if end < 0 {
				out.WriteString(query[i:])
				return out.String()
			}
			out.WriteString(query[i : i+end+1])
			i += end + 1
		case strings.HasPrefix(query[i:], "/*"):
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				out.WriteString(query[i:])
				return out.String()
			}
			out.WriteString(query[i : i+end+4])
			i += end + 4
		case query[i] == '$':
			end := i + 1
			for end < len(query) && (query[end] == '_' || query[end] >= 'a' && query[end] <= 'z' || query[end] >= 'A' && query[end] <= 'Z' || query[end] >= '0' && query[end] <= '9') {
				end++
			}
			if end < len(query) && query[end] == '$' {
				delimiter := query[i : end+1]
				closeAt := strings.Index(query[end+1:], delimiter)
				if closeAt >= 0 {
					finish := end + 1 + closeAt + len(delimiter)
					out.WriteString(query[i:finish])
					i = finish
					continue
				}
			}
			out.WriteByte(query[i])
			i++
		case query[i] == '?':
			parameter++
			out.WriteString(fmt.Sprintf("$%d", parameter))
			i++
		default:
			out.WriteByte(query[i])
			i++
		}
	}
	return out.String()
}
