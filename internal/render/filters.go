package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func init() {
	// Register custom pongo2 filters.

	pongo2.RegisterFilter("string", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		return pongo2.AsValue(fmt.Sprintf("%v", in.Interface())), nil
	})

	pongo2.RegisterFilter("strftime", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		// Convert Python strftime format to Go format.
		format := param.String()
		format = strings.ReplaceAll(format, "%Y", "2006")
		format = strings.ReplaceAll(format, "%m", "01")
		format = strings.ReplaceAll(format, "%d", "02")
		format = strings.ReplaceAll(format, "%H", "15")
		format = strings.ReplaceAll(format, "%M", "04")
		format = strings.ReplaceAll(format, "%S", "05")

		if t, ok := in.Interface().(*time.Time); ok && t != nil {
			return pongo2.AsValue(t.Format(format)), nil
		}
		if t, ok := in.Interface().(time.Time); ok {
			return pongo2.AsValue(t.Format(format)), nil
		}
		return pongo2.AsValue(""), nil
	})

	pongo2.RegisterFilter("objectid_hex", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		if oid, ok := in.Interface().(bson.ObjectID); ok {
			return pongo2.AsValue(oid.Hex()), nil
		}
		return pongo2.AsValue(""), nil
	})

	// is_in checks if a string value is present in a []string slice.
	// Usage: {% if value|is_in:slice %}
	pongo2.RegisterFilter("is_in", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		needle := in.String()
		if slice, ok := param.Interface().([]string); ok {
			for _, s := range slice {
				if s == needle {
					return pongo2.AsValue(true), nil
				}
			}
		}
		return pongo2.AsValue(false), nil
	})

	// replace replaces occurrences of a substring. Usage: {{ value|replace:"old,new" }}
	pongo2.RegisterFilter("replace", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		s := in.String()
		parts := strings.SplitN(param.String(), ",", 2)
		if len(parts) == 2 {
			s = strings.ReplaceAll(s, parts[0], parts[1])
		}
		return pongo2.AsValue(s), nil
	})

	// capitalize capitalizes the first letter of a string.
	pongo2.RegisterFilter("capitalize", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		s := in.String()
		if len(s) > 0 {
			s = strings.ToUpper(s[:1]) + s[1:]
		}
		return pongo2.AsValue(s), nil
	})

	// is_image checks if a filename has an image extension.
	pongo2.RegisterFilter("is_image", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		name := strings.ToLower(in.String())
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg"} {
			if strings.HasSuffix(name, ext) {
				return pongo2.AsValue(true), nil
			}
		}
		return pongo2.AsValue(false), nil
	})
}
