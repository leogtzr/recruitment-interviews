package main

import (
	"regexp"
	"testing"
)

func Test_sanitizeUserInput(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{"	alv", "alv"},
	}

	for _, tc := range tests {
		got := sanitizeUserInput(tc.input)
		if got != tc.want {
			t.Errorf("got=[%s], want=[%s]", got, tc.want)
		}
	}
}

func Test_userInputToCmd(t *testing.T) {
	type test struct {
		input string
		want  Command
	}

	tests := []test{
		{input: ":_", want: noCmd},
		{input: ":q", want: exitCmd},
		{input: "exit", want: exitCmd},
		{input: "use golang", want: useCmd},
	}

	for _, tc := range tests {
		got, _ := userInputToCmd(tc.input)
		if got != tc.want {
			t.Errorf("got=[%s], want=[%s]", got, tc.want)
		}
	}
}

func Test_questionHasValidFormat(t *testing.T) {
	rgx := regexp.MustCompile("^\\d+@.+@(\\d+)?$")
	type test struct {
		input string
		match bool
	}

	tests := []test{
		{input: "1@Cómo puedes sortear un archivo?@2", match: true},
		{input: "1@Cómo puedes sortear un archivo?@s", match: false},
		{input: "@Cómo puedes sortear un archivo?@s", match: false},
		{input: "1@x@2", match: true},
		{input: "1@@2", match: false},
	}
	for _, tc := range tests {
		match := isQuestionFormatValid(tc.input, rgx)
		if match != tc.match {
			t.Errorf("got=[%t], want=[%t]", match, tc.match)
		}
	}
}

func Test_toQuestion(t *testing.T) {
	type test struct {
		input    string
		question Question
	}

	tests := []test{
		{
			input:    "1@Cómo puedes sortear un archivo?@2",
			question: Question{ID: 1, Q: "Cómo puedes sortear un archivo?", NextQuestionID: 2, Answer: NotAnsweredYet},
		},
		{
			input:    "2@Cómo puedes obtener las ultimas 3 líneas de un archivo?@",
			question: Question{ID: 2, Q: "Cómo puedes obtener las ultimas 3 líneas de un archivo?", NextQuestionID: -1, Answer: NotAnsweredYet},
		},
	}

	for _, tc := range tests {
		got := toQuestion(tc.input)
		if got != tc.question {
			t.Errorf("got=[%s], want=[%s]", got, tc.question)
		}
	}
}