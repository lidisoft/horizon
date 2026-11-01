// Command schemajs writes the document schema as a JavaScript module, so
// the editor and the server share one definition (task gen:schema).
//
//	go run ./internal/document/schemajs web/app/editor/schema.js
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lidisoft/horizon/internal/document"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: schemajs <out.js>")
		os.Exit(2)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, bytes.TrimSpace(document.SchemaJSON()), "", "  "); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := "// Generated from internal/document/schema.json by task gen:schema. Do not edit.\n\n" +
		"export default " + pretty.String() + ";\n"
	if err := os.WriteFile(os.Args[1], []byte(out), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
