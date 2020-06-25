package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"github.com/spf13/viper"
)

func sanitizeUserInput(input string) string {
	return strings.TrimSpace(input)
}

// Transforms user's input to a Command
func userInputToCmd(input string) (Command, []string) {
	if len(input) == 0 {
		return noCmd, []string{}
	}
	fullCommand := words(input)
	input = fullCommand[0]
	input = sanitizeUserInput(input)
	input = strings.ToLower(input)
	switch input {
	case "exit", "quit", ":q", "/q", "q":
		return exitCmd, []string{}
	case "topics", "tps", "t", "/t", ":t":
		return topicsCmd, []string{}
	case "help", ":h", "/h", "--h", "-h", "h":
		return helpCmd, []string{}
	case "use", "u", "/u", ":u", "-u", "--u", "set":
		if len(fullCommand) <= 1 {
			return noCmd, []string{}
		}
		return useCmd, fullCommand[1:]
	case "cls", "clear":
		return clearScreenCmd, []string{}
	case "pwd":
		return pwdCmd, []string{}
	case "start", "begin":
		return startCmd, []string{}
	case "p", "print", "print()", "p()":
		return printCmd, []string{}
	case "next", "nxt", ">":
		return nextQuestionCmd, []string{}
	case "previous", "prev", "<":
		return previousQuestionCmd, []string{}
	case "view", "v":
		return viewCmd, []string{}
	case "y", "right", "ok", "yes", "si":
		return rightAnswerCmd, []string{}
	case "n", "no", "mal", "wrong", "nop", "bad", "nel":
		return wrongAnswerCmd, []string{}
	case "hmm", "meh", "?":
		return mehAnswerCmd, []string{}
	case "finish", "done", "bye":
		return finishCmd, []string{}
	// case "load":
	// 	return loadCmd, fullCommand[1:]
	case "exf":
		return exitInterviewFileCmd, []string{}
	case "+":
		return increaseLevelCmd, []string{}
	case "-":
		return decreaseLevelCmd, []string{}
	case "=":
		return ignoreLevelCmd, []string{}
	case "lvl":
		return showLevelCmd, []string{}
	case "stats":
		return showStatsCmd, []string{}
	case "ap":
		return setAssociateProgrammerLevelCmd, []string{}
	case "pa":
		return setProgrammerAnalystLevelCmd, []string{}
	case "sr":
		return setSRProgrammerLevelCmd, []string{}
		// TODO: this is not relevant anymore
	//case "validate", "val", "check":
	//return validateQuestionsCmd, []string{}
	case "count", "cnt", "c":
		return countCmd, []string{}
	case "nt", "notes":
		return notesCmd, []string{}
	}
	return noCmd, []string{}
}

func dirExists(dirPath string) bool {
	if _, err := os.Stat(dirPath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}

func exists(name string) bool {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false
	}
	return true
}

func getTopicsWithQuestions(db *sql.DB) ([]string, error) {
	var topics []string
	results, err := db.Query("select distinct(t.topic) from topic t inner join question q on t.id = q.topic_id")
	if err != nil {
		return []string{}, err
	}
	defer results.Close()

	for results.Next() {
		var topic string
		err = results.Scan(&topic)
		if err != nil {
			return []string{}, err
		}
		topics = append(topics, topic)
	}

	return topics, nil
}

func retrieveTopicsFromFileSystem(db *sql.DB) ([]string, error) {
	topics, err := getTopicsWithQuestions(db)
	if err != nil {
		return []string{}, err
	}
	return topics, err
}

func retrieveTopicsFromInterview(topics *map[string][]Question) []string {
	tps := make([]string, 0)
	for t := range *topics {
		tps = append(tps, t)
	}
	return tps
}

func getTopics(db *sql.DB) ([]Topic, error) {
	var topics []Topic
	results, err := db.Query("SELECT * FROM topic")
	if err != nil {
		return []Topic{}, err
	}
	defer results.Close()

	for results.Next() {
		var topic Topic
		err = results.Scan(&topic.ID, &topic.Topic)
		if err != nil {
			return []Topic{}, err
		}
		topics = append(topics, topic)
	}

	return topics, nil
}

func listTopics(db *sql.DB) error {
	topics, err := getTopics(db)
	if err != nil {
		return err
	}

	for _, topic := range topics {
		fmt.Println(termenv.String(topic.Topic).Underline().Bold())
	}
	return nil
}

func printHelp() {
	usage := `
commands:

	exit|quit|:q|/q|q 			exits from this application.
	topics|tps|t|/t|:t 			list current available topics from file system or a loaded interview.
	help|:h|/h|--h|-h 			shows this message.
	use|u|/u|:u|-u|--u|set 			sets an available topic.
	cls|clear 				clears the screen.
	pwd 					prints the current selected topic.
	start|begin 				starts the interview.
	print|print()|p|p() 			prints the current question.
	next|nxt|> 				moves to the next question.
	previous|prev|< 			moves to the previous question.
	view|v					prints the current available questions by level.
	no|n|mal|wrong|nop|bad|nel 		marks a question as wrong.
	ok|yes|si|right|y			marks a question as right / OK.
	hmm|meh|?				marks a question as neutral.
	finish|done|bye				finishes an interview.
	+					increases the level of the interview, it could be from Programmer Analyst to a Sr Programmer Analyst as an example.
	- 					decreases the level of the interview.
	= 					ignore levels.
	lvl					prints the current interview level.
	stats					shows some stats and the current configuration for the interview.
	ap					sets the level of the interview to "Associate Programmer"
	pa					sets the level of the interview to "Programmer Analyst"
	sr					sets the level of the interview to "Sr Programmer Analyst"


	Any other command or sentence that is not listed here will be simply ignored.
	`

	fmt.Println(usage)
}

func words(input string) []string {
	return strings.Fields(input)
}

func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func topicExist(topic string, topics *[]string) bool {
	r := false

	for _, t := range *topics {
		if t == topic {
			r = true
			break
		}
	}

	return r
}

func toQuestion(question string) Question {
	questionFields := strings.Split(question, "@")
	id, _ := strconv.ParseInt(questionFields[0], 10, 64)
	q := questionFields[1]
	level, _ := strconv.ParseInt(questionFields[2], 10, 64)
	return Question{ID: int(id), Q: q, Answer: NotAnsweredYet, Level: Level(level)}
}

func extractTopicName(options []string) string {
	topicName := options[0]
	topicName = strings.ToLower(topicName)
	return topicName
}

func setTopic(options []string, config *Config, db *sql.DB) error {
	topicName := extractTopicName(options)
	topics, err := getTopicsWithQuestions(db)
	if err != nil {
		return err
	}

	if topicExist(topicName, &topics) {
		config.selectedTopic = topicName
		questionsPerTopic, err := loadQuestionsFromTopic(config, db)
		if err != nil {
			return err
		}
		config.interview.Topics[config.selectedTopic] = questionsPerTopic
	} else {
		fmt.Println(
			termenv.String(fmt.Sprintf("topic '%s' not found or the topic selected doesn't have questions.", topicName)).Foreground(config.colorProfile.Color(red)))
	}
	return nil
}

func saveIntervieweeName(interviewee string, db *sql.DB) error {
	stmt, err := db.Prepare("insert into candidate(name) values(?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(interviewee)
	if err != nil {
		return err
	}
	return nil
}

func getQuestionsByTopic(topic string, db *sql.DB) ([]Question, error) {
	questionsPerTopic := make([]Question, 0)

	results, err :=
		db.Query(
			`select q.id, question, q.level_id from question q, topic t where t.topic = ? and t.id = q.topic_id`,
			topic)
	if err != nil {
		return []Question{}, err
	}
	defer results.Close()

	for results.Next() {
		var question Question
		err = results.Scan(&question.ID, &question.Q, &question.Level)
		if err != nil {
			return []Question{}, err
		}
		questionsPerTopic = append(questionsPerTopic, question)
	}

	return questionsPerTopic, nil
}

func loadQuestionsFromTopic(config *Config, db *sql.DB) ([]Question, error) {
	// Clear previous questions ...
	questionsPerTopic, err := getQuestionsByTopic(config.selectedTopic, db)
	if err != nil {
		return []Question{}, err
	}

	levelFound := findLevel(&questionsPerTopic, AssociateOrProgrammer, ProgrammerAnalyst, SrProgrammer)
	fmt.Printf("Loaded -> '%d' questions, starting with: %s level.\n", len(questionsPerTopic), levelFound)

	levelQCounts := levelQuestionCounts(&questionsPerTopic)
	fmt.Printf("Associate Programmer = ")
	printWithColorf(config, "%d\n", green, levelQCounts[AssociateOrProgrammer])
	fmt.Printf("Programmer Analyst = ")
	printWithColorf(config, "%d\n", green, levelQCounts[ProgrammerAnalyst])
	fmt.Printf("Sr. Programmer  = ")
	printWithColorf(config, "%d\n", green, levelQCounts[SrProgrammer])

	return questionsPerTopic, nil
}

func setTopicFrom(inputOptions []string, topicsFromInterviewFile *map[string][]Question, config *Config) {
	topicName := extractTopicName(inputOptions)
	topics := retrieveTopicsFromInterview(topicsFromInterviewFile)
	if topicExist(topicName, &topics) {
		config.selectedTopic = topicName
		return
	}

	fmt.Println(
		termenv.String(fmt.Sprintf("topic '%s' not found or the topic selected doesn't have questions.", topicName)).Foreground(config.colorProfile.Color(red)))
}

func shouldIgnoreLine(line string) bool {
	return strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0
}

func levelQuestionCounts(qs *[]Question) map[Level]int {
	counts := make(map[Level]int)
	for _, q := range *qs {
		counts[q.Level]++
	}
	return counts
}

func shortIntervieweeName(name string, min int) string {
	if len(name) == 0 {
		return "(who?)"
	}
	if len(name) < min {
		return fmt.Sprintf("(%s)", name)
	}
	return fmt.Sprintf("(%s...)", name[0:min])
}

func ps1String(ps1, selectedTopic, intervieweeName string) string {
	if selectedTopic == "" {
		return "$ "
	}
	return fmt.Sprintf(
		"/%s %s $ ",
		termenv.String(selectedTopic).Faint(), shortIntervieweeName(intervieweeName, minNumberOfCharsInIntervieweeName))
}

func isQuestionFormatValid(question string, rgx *regexp.Regexp) bool {
	return rgx.MatchString(question)
}

func (q Question) String() string {
	return fmt.Sprintf("Q%d: %s [%s] [%s]", q.ID, q.Q, q.Answer, q.Level)
}

func printQuestion(questionIndex int, config *Config) {
	if !config.hasStarted {
		return
	}

	if config.ignoreLevelChecking && (len(config.interview.Topics[config.selectedTopic]) > 0) {
		fmt.Println(config.interview.Topics[config.selectedTopic][config.questionIndex])
		fmt.Println()
		return
	}
	currentLevel := config.levels[config.levelIndex]
	currentLevelQuestions := getQuestionsFromLevel(currentLevel, config)
	if len(currentLevelQuestions) == 0 {
		printWithColorln("There are no questions for this level.", yellow, config)
		fmt.Println()
		return
	}
	index := config.individualLevelIndexes[int(currentLevel)-1]
	fmt.Println(currentLevelQuestions[index])
	fmt.Println()
}

func viewQuestions(config *Config) {
	if len(config.interview.Topics[config.selectedTopic]) < 1 {
		printWithColorln("You need to select a topic first.", red, config)
		fmt.Println()
		return
	}
	for _, q := range config.interview.Topics[config.selectedTopic] {
		fmt.Println(q)
	}
}

func viewQuestionsByLevel(config *Config) {
	if len(config.selectedTopic) == 0 {
		printWithColorln("You need to select a topic first.", red, config)
		return
	}
	currentLevel := config.levels[config.levelIndex]
	currentLevelQuestions := getQuestionsFromLevel(currentLevel, config)
	for _, q := range currentLevelQuestions {
		fmt.Println(q)
	}
}

func readIntervieweeName(stdin io.Reader) (string, bool) {
	reader := bufio.NewScanner(stdin)
	reader.Scan()
	text := reader.Text()
	if len(strings.TrimSpace(text)) == 0 {
		return "", false
	}
	return strings.TrimSpace(text), true
}

func printWithColorln(msg, colorCode string, config *Config) {
	fmt.Println(termenv.String(msg).Foreground(config.colorProfile.Color(colorCode)))
}

func printWithColorf(config *Config, msg, colorCode string, a ...interface{}) {
	fmt.Printf(termenv.String(msg).Foreground(config.colorProfile.Color(colorCode)).String(), a...)
}

func saveInterview(config *Config) error {
	intervieweeName := config.interview.Interviewee
	savedDir := filepath.Join(config.interviewTopicsDir, "saved")
	if !dirExists(savedDir) {
		return fmt.Errorf("[%s] does not exist", savedDir)
	}

	savedInterviewName := filepath.Join(savedDir, intervieweeName)
	if dirExists(savedInterviewName) {
		printWithColorln(fmt.Sprintf("[%s] already exists, we will generate another name.", savedInterviewName), red, config)
		seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
		randDirName := stringWithCharset(2, charset, seededRand)
		savedInterviewName = fmt.Sprintf("%s-%s", savedInterviewName, randDirName)
	}
	if err := os.MkdirAll(savedInterviewName, os.ModePerm); err != nil {
		return err
	}
	return saveData(filepath.Join(savedInterviewName, "interview"), config.interview)
}

func saveData(savedInterviewNamePath string, interview Interview) error {
	file, err := os.Create(savedInterviewNamePath)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	fmt.Fprintf(w, "%s@%s\n", interview.Interviewee, interview.Date.Format(interviewFormatLayout))

	for topicName, questions := range interview.Topics {
		for _, q := range questions {
			if q.Answer != NotAnsweredYet {
				fmt.Fprintf(w, "%s@%d@%s@%d\n", topicName, q.ID, q.Q, int(q.Answer))
			}
		}
	}
	return w.Flush()
}

/*
func loadInterview(options []string, config *Config) (Interview, error) {
	interviewName := strings.Join(options, " ")
	interviewFile := filepath.Join(config.interviewTopicsDir, "saved", interviewName, "interview")
	if !dirExists(interviewFile) {
		return Interview{}, fmt.Errorf("'%s' does not exist", interviewFile)
	}
	file, err := os.Open(interviewFile)
	if err != nil {
		return Interview{}, err
	}
	defer file.Close()

	interview := Interview{}

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		header := scanner.Text()
		intervieweeName, err := extractNameFromInterviewHeaderRecord(header)
		if err != nil {
			return Interview{}, err
		}
		interview.Interviewee = intervieweeName

		interviewDate, err := extractDateFromInterviewHeaderRecord(header)
		if err != nil {
			return Interview{}, err
		}
		interview.Date = interviewDate
	}

	interview.Topics = make(map[string][]Question)

	// Load questions:
	for scanner.Scan() {
		questionFileRecord := scanner.Text()
		topic, question := extractQuestionInfo(questionFileRecord)
		interview.Topics[topic] = append(interview.Topics[topic], question)
	}

	return interview, nil
}
*/

// func extractNameFromInterviewHeaderRecord(header string) (string, error) {
// 	fields := strings.Split(strings.TrimSpace(header), "@")
// 	if len(fields) != requiredNumberOfFieldsInInterviewHeaderRecord {
// 		return "", fmt.Errorf("'%s' wrong header format", header)
// 	}
// 	return fields[0], nil
// }

func extractDateFromInterviewHeaderRecord(header string) (time.Time, error) {
	fields := strings.Split(strings.TrimSpace(header), "@")
	if len(fields) != requiredNumberOfFieldsInInterviewHeaderRecord {
		return time.Time{}, fmt.Errorf("'%s' wrong header format", header)
	}
	interviewDate, err := time.Parse(interviewFormatLayout, fields[1])
	return interviewDate, err
}

func extractQuestionInfo(questionFileRecord string) (string, Question) {
	fields := strings.Split(questionFileRecord, "@")
	topic := fields[0]
	id, _ := strconv.ParseInt(fields[1], 10, 64)
	question := fields[2]

	q := Question{ID: int(id), Q: question}
	x, _ := strconv.ParseInt(fields[4], 10, 64)
	q.Answer = Answer(int(x))

	return topic, q
}

func resetStatus(config *Config) {
	config.interview = Interview{Topics: make(map[string][]Question)}
	//config.usingInterviewFile = false
	config.hasStarted = false
	config.questionIndex = 0
	config.selectedTopic = ""
	config.ps1 = "$ "
}

func showLevel(config *Config) {
	currentLevel := config.levels[config.levelIndex]
	printWithColorln(currentLevel.String(), cyan, config)
}

func setAnswerAsNeutral(questions *[]Question, config *Config) {
	(*questions)[config.questionIndex].Answer = Neutral
	printWithColorln(fmt.Sprintf("Answer has saved as '%s'", Neutral), magenta, config)
}

func setAnswerAsOK(questions *[]Question, config *Config) {
	(*questions)[config.questionIndex].Answer = OK
	printWithColorln(fmt.Sprintf("Answer has saved as '%s'", OK), green, config)
}

func answerAs(config *Config, ans Answer, messageColorCode string) {
	currentLevel := config.levels[config.levelIndex]
	currentLevelQuestions := getQuestionsFromLevel(currentLevel, config)
	index := config.individualLevelIndexes[int(currentLevel)-1]
	id := currentLevelQuestions[index].ID
	qs := config.interview.Topics[config.selectedTopic]
	markQuestionAs(id, ans, &qs)
	printWithColorln(fmt.Sprintf("Answer has saved as '%s'", ans), messageColorCode, config)
}

func setAnswerAsWrong(questions *[]Question, config *Config) {
	(*questions)[config.questionIndex].Answer = Wrong
	printWithColorln(fmt.Sprintf("Answer has saved as '%s'", Wrong), red, config)
}

func markQuestionAs(id int, ans Answer, qs *[]Question) {
	for _, q := range *qs {
		if q.ID == id {
			(*qs)[id-1].Answer = ans
			break
		}
	}
}

func showStats(config *Config) {
	currentLevel := config.levels[config.levelIndex]

	if len(config.selectedTopic) == 0 {
		fmt.Printf("Level: ")
		printWithColorf(config, "%s\n", green, currentLevel)

		fmt.Printf("Ignoring level: ")
		printWithColorf(config, "%t\n", green, config.ignoreLevelChecking)

		fmt.Printf("Questions in bucket: ")
		printWithColorf(config, "%t\n", green, len(config.selectedTopic) != 0)
	} else {
		counts := countGeneral(&config.interview.Topics)
		notAnsweredCount := counts[NotAnsweredYet]
		okCount := counts[OK]
		wrongCount := counts[Wrong]
		neutralCount := counts[Neutral]
		total := notAnsweredCount + okCount + wrongCount + neutralCount

		fmt.Printf("Level: ")
		printWithColorf(config, "%s\n", green, currentLevel)

		fmt.Printf("Ignoring level: ")
		printWithColorf(config, "%t\n", green, config.ignoreLevelChecking)

		fmt.Printf("Questions in bucket: ")
		printWithColorf(config, "%t\n", green, len(config.selectedTopic) != 0)

		fmt.Printf("Not Answered: ")
		printWithColorf(config, "%d (%.2f%%)\n", green, notAnsweredCount, perc(notAnsweredCount, total))

		fmt.Printf("OK: ")
		printWithColorf(config, "%d (%.2f%%)\n", green, okCount, perc(okCount, total))

		fmt.Printf("Wrong: ")
		printWithColorf(config, "%d (%.2f%%)\n", green, wrongCount, perc(wrongCount, total))

		fmt.Printf("Neutral: ")
		printWithColorf(config, "%d (%.2f%%)\n", green, neutralCount, perc(neutralCount, total))
	}
}

func count(questions *[]Question, ans Answer) int {
	c := 0
	for _, q := range *questions {
		if q.Answer == ans {
			c++
		}
	}
	return c
}

func perc(count, total int) float64 {
	return (float64(count) * 100.0) / float64(total)
}

func countGeneral(topics *map[string][]Question) map[Answer]int {
	counts := make(map[Answer]int, 0)

	// flat the questions ...
	questions := make([]Question, 0)
	for _, qs := range *topics {
		for _, q := range qs {
			questions = append(questions, q)
		}
	}

	counts[NotAnsweredYet] = count(&questions, NotAnsweredYet)
	counts[OK] = count(&questions, OK)
	counts[Wrong] = count(&questions, Wrong)
	counts[Neutral] = count(&questions, Neutral)

	return counts
}

func setLevel(lvl Level, config *Config) {
	config.levelIndex = int(lvl) - 1
	currentLevel := config.levels[config.levelIndex]
	fmt.Printf("Current level is: ")
	printWithColorln(fmt.Sprintf("%s", currentLevel), green, config)
}

func validateQuestions(config *Config) {
	topicsDir := filepath.Join(config.interviewTopicsDir, "topics")

	if !dirExists(topicsDir) {
		log.Fatalf("'%s' does not exist", topicsDir)
	}
	err := filepath.Walk(topicsDir, func(path string, info os.FileInfo, err error) error {
		if !exists(filepath.Join(path, "questions")) {
			return nil
		}
		path = filepath.Base(path)
		if path == "topics" || path == "questions" {
			return nil
		}
		questionFile := filepath.Join(topicsDir, path, "questions")
		if has, lineNumbers := hasErrors(questionFile, config); has {
			fmt.Printf("%s has errors, lines:\n", questionFile)
			for _, line := range lineNumbers {
				fmt.Printf("\t%d\n", line)
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func hasErrors(interviewFilePath string, config *Config) (bool, []int) {
	has := false
	lineNumbers := []int{}
	file, err := os.Open(interviewFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	questionIndex := 0
	for scanner.Scan() {
		questionIndex++
		questionText := scanner.Text()
		if shouldIgnoreLine(questionText) {
			continue
		}
		if !isQuestionFormatValid(questionText, &config.rgxQuestions) {
			has = true
			lineNumbers = append(lineNumbers, questionIndex)
		}
	}
	return has, lineNumbers
}

func showCounts(config *Config) {
	qs := config.interview.Topics[config.selectedTopic]
	levelQCounts := levelQuestionCounts(&qs)
	fmt.Printf("Associate Programmer = ")
	printWithColorf(config, "%d\n", green, levelQCounts[AssociateOrProgrammer])
	fmt.Printf("Programmer Analyst = ")
	printWithColorf(config, "%d\n", green, levelQCounts[ProgrammerAnalyst])
	fmt.Printf("Sr. Programmer  = ")
	printWithColorf(config, "%d\n", green, levelQCounts[SrProgrammer])
}

// NewConfig Creates a new Configuration object.
func NewConfig() Config {
	cfg := Config{}
	cfg.rgxQuestions = *regexp.MustCompile("^\\d+@.+@(\\d+)?$")
	cfg.selectedTopic = ""
	cfg.ps1 = "$ "
	cfg.colorProfile = termenv.ColorProfile()
	cfg.interview = Interview{Topics: make(map[string][]Question)}
	cfg.topicQuestionsLevel = AssociateOrProgrammer
	cfg.levelIndex = 0
	cfg.ignoreLevelChecking = false
	cfg.individualLevelIndexes = []int{0, 0, 0}
	// cfg.usingInterviewFile = false
	cfg.questionIndex = 0
	cfg.levels = [3]Level{
		AssociateOrProgrammer, ProgrammerAnalyst, SrProgrammer,
	}
	return cfg
}

func createNotes(config *Config) error {
	if !config.hasStarted {
		return fmt.Errorf("Interview hasn't started")
	}
	intervieweeName := config.interview.Interviewee
	savedDir := filepath.Join(config.interviewTopicsDir, "saved")
	if !dirExists(savedDir) {
		return fmt.Errorf("[%s] does not exist", savedDir)
	}
	savedInterviewName := filepath.Join(savedDir, intervieweeName)
	if !dirExists(savedInterviewName) {
		err := os.MkdirAll(savedInterviewName, os.ModePerm)
		if err != nil {
			return err
		}
	}

	notesFilePath := filepath.Join(savedInterviewName, "notes.txt")
	file, err := os.OpenFile(notesFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	fmt.Fprintf(w, "Notes:\n\n")
	w.Flush()

	return openOSEditor(runtime.GOOS, notesFilePath)
}

func openOSEditor(osVersion, notesFile string) error {
	var cmd *exec.Cmd
	oldStdout, oldStdin, oldSterr := os.Stdout, os.Stdin, os.Stderr
	if osVersion == "windows" {
		cmd = exec.Command("notepad", notesFile)
	} else {
		cmd = exec.Command("/usr/bin/xterm", "-fa", "Monospace", "-fs", "14", "-e", "/usr/bin/vim", "+$", notesFile)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	os.Stdout, os.Stdin, os.Stderr = oldStdout, oldStdin, oldSterr
	return err
}

func readConfig(filename, configPath string, defaults map[string]interface{}) (*viper.Viper, error) {
	v := viper.New()
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
	v.SetConfigName(filename)
	v.AddConfigPath(configPath)
	v.SetConfigType("env")
	err := v.ReadInConfig()
	return v, err
}
