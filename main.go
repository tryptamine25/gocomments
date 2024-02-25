package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
)

const (
	openComment       = "/*"
	closeComment      = "*/"
	singleLineComment = "//"
)

type FileResult struct {
	FilePath           string
	CodeLines          int
	MultilineComments  int
	SingleLineComments int
	Error              error
}

func main() {
	fmt.Print("Enter the path to the folder: ")
	var folderPath string
	fmt.Scanln(&folderPath)

	files, err := getTSFilesInFolder(folderPath)
	if err != nil {
		log.Fatal(err)
	}

	var (
		totalCodeLines          = 0
		totalMultilineComments  = 0
		totalSingleLineComments = 0
		totalProcessedLines     = 0
		filesProcessed          = 0
		wg                      sync.WaitGroup
		resultCh                = make(chan *FileResult, len(files))
	)

	for _, file := range files {
		wg.Add(1)
		go analyzeFileAsync(file, &wg, resultCh)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		if result.Error != nil {
			log.Printf("Error during file analysis %s: %v\n", result.FilePath, result.Error)
			continue
		}
		totalCodeLines += result.CodeLines
		totalMultilineComments += result.MultilineComments
		totalSingleLineComments += result.SingleLineComments
		totalProcessedLines += result.CodeLines + result.MultilineComments + result.SingleLineComments
		filesProcessed++
	}
	percentageTotalComments := float64(totalMultilineComments+totalSingleLineComments) / float64(totalCodeLines) * 100

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Parameter", "Value"})
	table.Append([]string{"Number of files", fmt.Sprintf("%d", filesProcessed)})
	table.Append([]string{"Total processed lines", fmt.Sprintf("%d", totalProcessedLines)})
	table.Append([]string{"Total number of lines of code", fmt.Sprintf("%d", totalCodeLines)})
	table.Append([]string{"Total number of comments lines", fmt.Sprintf("%d", totalMultilineComments+totalSingleLineComments)})
	table.Append([]string{"Percentage of total comments lines", fmt.Sprintf("%.2f%%", percentageTotalComments)})

	table.Render()
}

func analyzeFileAsync(filePath string, wg *sync.WaitGroup, resultCh chan<- *FileResult) {
	defer wg.Done()

	codeLines, multilineComments, singleLineComments, err := analyzeFile(filePath)
	resultCh <- &FileResult{
		FilePath:           filePath,
		CodeLines:          codeLines,
		MultilineComments:  multilineComments,
		SingleLineComments: singleLineComments,
		Error:              err,
	}
}

func getTSFilesInFolder(folderPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !shouldExcludeDir(path) && (strings.HasSuffix(info.Name(), ".tsx") || strings.HasSuffix(info.Name(), ".ts")) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func shouldExcludeDir(path string) bool {
	excludedDirs := []string{"node_modules", "dist"}
	for _, dir := range excludedDirs {
		if strings.Contains(path, dir) {
			return true
		}
	}
	return false
}

func analyzeFile(filePath string) (int, int, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	var (
		lineNumber             = 0
		multilineCommentLines  = 0
		singleLineCommentLines = 0
		inMultilineComment     bool
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		containOpens := strings.Contains(line, openComment)
		containCloses := strings.Contains(line, closeComment)
		containSingleLineComment := strings.Contains(line, singleLineComment)

		if containOpens && containCloses {
			multilineCommentLines++
		} else if containOpens {
			inMultilineComment = true
			multilineCommentLines++
		} else if containCloses {
			if inMultilineComment {
				multilineCommentLines++
				inMultilineComment = false
			}
		} else if inMultilineComment {
			multilineCommentLines++
		}

		if containSingleLineComment {
			singleLineCommentLines++
		}
	}

	codeLines := lineNumber - multilineCommentLines - singleLineCommentLines

	return codeLines, multilineCommentLines, singleLineCommentLines, nil
}
