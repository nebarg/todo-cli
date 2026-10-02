package scan

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestFileTodosReadOnlyComments(t *testing.T) {
	for _, c := range []struct {
		path    string
		content []string
		want    []string // "line: note"
	}{
		{"code.ts", []string{
			"class TodoItems {}",
			"class Todo {",
			"const todo = new Todo();",
			`let msg = "TODO: not a comment";`,
			`let url = "http://example.com"; // TODO fix the url`,
			"// see the todo list for details",
			"fetchTodos(); // load todos",
			"/*",
			"  TODO inside a block comment without stars",
			"*/",
			"/**",
			" * TODO with stars",
			" */",
			"const page = `",
			"  <a href=\"http://x\">// TODO: inside a template string</a>",
			"`;",
			"a(); /* note */ b(); // TODO: second comment on the line",
			"/* TODO: closed */ code(); // and then code",
		}, []string{"5: fix the url", "9: inside a block comment without stars", "12: with stars", "17: second comment on the line", "18: closed"}},
		{"style.css", []string{
			"#todo { color: red; }",
			"#todo-list { background: url(http://x/todo.png); }",
			"/* TODO tidy colours */",
		}, []string{"3: tidy colours"}},
		{"macro.c", []string{
			"#define TODO(x) x",
			`#include "todo.h"`,
			"int x; // todo0 ship it",
		}, []string{"3: ship it"}},
		{"script.py", []string{
			"def todo(): pass",
			`"""`,
			"# TODO: inside a docstring is not a comment",
			`"""`,
			"x = 1  # TODO handle errors",
			"y = 'a # TODO: in a string'",
		}, []string{"5: handle errors"}},
		{"lib.rs", []string{
			"#[derive(Debug)] // TODO: derive more",
			"fn f<'a>(x: &'a str) { todo!() } // todo1 lifetimes",
		}, []string{"2: lifetimes", "1: derive more"}},
		{"page.php", []string{
			"#[Todo('later')]",
			"# TODO: hash comment",
			"$x = '#todo'; // TODO(gb): slash comment",
			"?><p>Basket</p><!-- TODO: html comment --><?php",
		}, []string{"2: hash comment", "3: slash comment", "4: html comment"}},
		{"run.sh", []string{
			`echo "$#" # TODO: count arguments`,
			"n=${#todo}",
		}, []string{"1: count arguments"}},
		{"query.sql", []string{
			"SELECT '-- TODO: in a string';",
			"-- TODO: check int size for id",
		}, []string{"2: check int size for id"}},
		{"init.lua", []string{
			"--[[",
			"TODO: inside a block",
			"]]",
			"x = 1 -- todo00 urgent",
		}, []string{"4: urgent", "2: inside a block"}},
		{"form.blade.php", []string{
			"{{-- todo@dark-mode: input styling --}}",
			"<input class=\"todo\">",
		}, []string{"1: input styling"}},
		{"list.twig", []string{"{# TODO: paginate #}", "{{ todo.name }}"}, []string{"1: paginate"}},
		{"list.tpl", []string{"{* TODO: paginate *}", "{$todo.name}"}, []string{"1: paginate"}},
		{"config.yaml", []string{
			"url: http://example.com/#TODO(x)",
			"name: todo",
			"retries: 3 # TODO: tune",
		}, []string{"3: tune"}},
		{"notes.html", []string{
			"<p>TODO: plain text, and don't stop at the apostrophe</p> <!-- TODO: in a comment -->",
			"<script>",
			"  //TODO: write me!",
			"  /**",
			"   * @todo PSA-1062 add a search field",
			"   */",
			"</script>",
			"<p>See http://example.com/todo (docs)</p>",
		}, []string{"1: in a comment", "3: write me!", "5: PSA-1062 add a search field"}},
		{"ProcessHeader.tpl", []string{"<script>", "  //@TODO: load page disabler", "</script>", "{* TODO: tidy *}"}, []string{"2: load page disabler", "4: tidy"}},
		{"orders.mysql", []string{"<?php", "  // TODO: T6571 Move into PayerAuth class", "  $q = 'SELECT 1'; -- not a PHP comment todo"}, []string{"2: T6571 Move into PayerAuth class"}},
		{"entity.xml", []string{"<!--", "TODO - consider stripping out old entities", "-->", "<entity name=\"todo\"/>"}, []string{"2: consider stripping out old entities"}},
		{"schema.sql", []string{"SELECT 1; -- TODO: index this", "SELECT 'http://x'; // not a SQL comment todo"}, []string{"1: index this"}},
		{"unknown.zz", []string{"code // TODO: guessed", "text todo"}, []string{"1: guessed"}},
		{"open.js", []string{"const s = 'it", "// TODO: a stray quote ends with its line"}, []string{"2: a stray quote ends with its line"}},
	} {
		t.Run(c.path, func(t *testing.T) {
			var got []string
			for _, m := range sortedMatches(fileTodos(c.path, strings.Join(c.content, "\n"))) {
				got = append(got, fmt.Sprintf("%d: %s", m.Line, m.Note))
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestFileTodosKeepLevelsAndCategories(t *testing.T) {
	matches := fileTodos("a.go", "x := 1 // todo00 first\n/* todo@boundary\n   split */\n")
	if len(matches) != 2 || matches[0].Level != "00" || matches[1].Category != "boundary" || matches[1].Line != 2 {
		t.Fatalf("matches = %+v", matches)
	}
	if matches := fileTodos("a.go", "package a\n\nfunc A() {}\n"); matches != nil {
		t.Fatalf("a file without a marker = %+v", matches)
	}
}

// BenchmarkFileTodos reads a large PHP class with a few to-dos, the common
// case for a file containsTodo lets through.
func BenchmarkFileTodos(b *testing.B) {
	var file strings.Builder
	file.WriteString("<?php\nclass Orders {\n")
	for i := range 5000 {
		fmt.Fprintf(&file, "\t/**\n\t * Loads order %d from \"the database\".\n\t */\n\tfunction load%d($id) { return $this->db->query('SELECT * FROM orders WHERE id = ' . (int) $id); } // http://example.com\n", i, i)
		if i%1000 == 0 {
			file.WriteString("\t// TODO: cache this\n")
		}
	}
	file.WriteString("}\n")
	content := file.String()
	b.SetBytes(int64(len(content)))
	for b.Loop() {
		if len(fileTodos("Orders.class.php", content)) != 5 {
			b.Fatal("wrong number of to-dos")
		}
	}
}
