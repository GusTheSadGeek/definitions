package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"reflect"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

type Dict map[interface{}]interface{}

// This can be improved - maybe use string.builder and/or regex
// Just puts a '.' infront of text inside {{...}}
// Ignores {%....%}
func convertFromJinja(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	for lineNumber := range lines {
		startIndex := 0
		leftIndex := 0
		for leftIndex > -1 {
			line := lines[lineNumber]
			leftIndex = bytes.Index(line[startIndex:], []byte("{{"))
			if leftIndex > -1 {
				rightIndex := bytes.Index(line[startIndex+leftIndex:], []byte("}}"))
				if rightIndex > -1 {
					variableName := bytes.TrimLeft(line[startIndex+leftIndex+2:startIndex+leftIndex+rightIndex], " ")
					newLine := make([]byte, 0) // I wanted to make this len(line)+2 but then append just added to end of it all
					newLine = append(newLine, line[:startIndex+leftIndex+2]...)
					newLine = append(newLine, byte('.'))
					newLine = append(newLine, variableName...)
					newLine = append(newLine, line[startIndex+leftIndex+rightIndex:]...)
					lines[lineNumber] = newLine
				}
				startIndex = startIndex + leftIndex + 3
			}
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

// Load definitions file
// adds all !include <files>
func loadDefFiles(filename string, alreadyIncluded []string) ([]byte, error) {

	if alreadyIncluded == nil {
		alreadyIncluded = make([]string, 0)
	}

	currentDir, _ := path.Split(filename)

	outLines := make([][]byte, 0)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("%v\n", err)
		return nil, err
	}

	seperator := []byte("#!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	outLines = append(outLines, seperator)
	outLines = append(outLines, []byte(fmt.Sprintf("#!! %s !!", string(filename))))
	outLines = append(outLines, seperator)

	data = convertFromJinja(data)

	includeList := make([]string, 0)

	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("!include ")) {
			newFilename := string(line[len("!include "):])

			dir, f := path.Split(newFilename)

			if len(dir) == 0 {
				newFilename = path.Join(currentDir, f)
			} else {
				newFilename = path.Join(currentDir, newFilename)
			}

			includeList = append(includeList, newFilename)
		} else {
			outLines = append(outLines, line)
		}
	}

	// Check for nested includes
	for index, file := range includeList {
		for _, a := range alreadyIncluded {
			if a == file {
				log.Printf("File included twice [%s]", file)
				includeList[index] = ""
			}
		}
	}
	// update included list
	for _, file := range includeList {
		if len(file) > 0 {
			alreadyIncluded = append(alreadyIncluded, file)
		}
	}
	// finally include the included files
	for _, file := range includeList {
		// Recursive
		if len(file) > 0 {
			includedContent, err := loadDefFiles(file, alreadyIncluded)
			if err != nil {
				fmt.Printf("%v\n", err)
				return nil, err
			}
			outLines = append(outLines, includedContent)
		}
	}

	return bytes.Join(outLines, []byte("\n")), nil
}

func dumpDict(d Dict, filename string) error {
	out, err := yaml.Marshal(d)
	if err != nil {
		log.Fatalf("dumpDict - error: %v", err)
		return err
	}
	if len(filename) > 0 {
		err := ioutil.WriteFile(filename, out, 0644)
		if err != nil {
			fmt.Printf("%v\n", err)
			return err
		}
	} else {
		fmt.Printf("%v", string(out))
	}
	return nil
}

// Processes a template using the supplied dictionay
// Returns updated template content
func processTemplate(content []byte, dataDict Dict) []byte {
	tpl := template.New("template")
	tpl, err := tpl.Parse(string(content))
	if err != nil {
		log.Fatalf("processTemplate - error: %v", err)
	}

	var newContentBuffer bytes.Buffer
	err = tpl.Execute(&newContentBuffer, dataDict)
	if err != nil {
		log.Fatalf("processTemplate - error: %v", err)
	}
	return newContentBuffer.Bytes()
}

type Params struct {
	Filename          string
	OutputFilename    string
	ProcessedFilename string
}

var params Params

func initParams() {
	var filename, outputFilename, processedFilename string
	flag.StringVar(&filename, "inFile", "", "Definitions YAML file")
	flag.StringVar(&filename, "f", "", "Definitions YAML file")

	flag.StringVar(&outputFilename, "outFile", "", "Dictionary YAML file")
	flag.StringVar(&outputFilename, "o", "", "Dictionary YAML file")

	flag.StringVar(&processedFilename, "pFile", "", "Processed input file(s)")
	flag.StringVar(&processedFilename, "p", "", "Processed input file(s)")

	flag.Parse()

	params.Filename = filename
	params.OutputFilename = outputFilename
	params.ProcessedFilename = processedFilename
}

func main() {
	initParams()

	var newData []byte
	var data []byte
	var err error

	dataDictionay := make(Dict)

	// Get the templates - including !include <filename>
	newData, err = loadDefFiles(params.Filename, nil)

	if len(params.ProcessedFilename) > 0 {
		err := ioutil.WriteFile(params.ProcessedFilename, newData, 0644)
		if err != nil {
			fmt.Printf("Writing processedFile: %v\n", err)
		}
	}

	for !reflect.DeepEqual(data, newData) {
		data = newData
		err = yaml.Unmarshal([]byte(data), &dataDictionay)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		newData = processTemplate(data, dataDictionay)
	}

	dumpDict(dataDictionay, params.OutputFilename)
}
