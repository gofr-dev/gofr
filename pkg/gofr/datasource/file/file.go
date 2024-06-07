package file

import (
	"bufio"
	"encoding/json"
	"errors"
	"gofr.dev/pkg/gofr/datasource"
	"os"
	"strings"
)

type file struct {
	*os.File
	logger datasource.Logger
}

type textCSVReader struct {
	scanner *bufio.Scanner
	logger  datasource.Logger
}

type jsonReader struct {
	decoder *json.Decoder
	token   json.Token
}

// ReadAll reads either json, csv or text files, file with multiple rows, objects or single object can be read
// in the same way.
// File format is decided based on the extension
// JSON files are read in struct, while CSV files are read in pointer to string.
//
// newCsvFile, _ = fileStore.Open("file.csv")
// reader := newCsvFile.ReadAll()
//
// Reading JSON files
//
//	for reader.Next() {
//			var u User
//
//			reader.Scan(&u)
//		}
//
// Reading CSV files
//
//	for reader.Next() {
//			var content string
//
//			reader.Scan(&u)
//		}
func (f file) ReadAll() (datasource.RowReader, error) {
	if strings.HasSuffix(f.File.Name(), ".json") {
		decoder := json.NewDecoder(f.File)

		testDecoder := *decoder

		newDecoder := &testDecoder

		var token json.Token

		t, err := newDecoder.Token()
		if err != nil {
			f.logger.Errorf("failed to decode JSON token %v", err)

			return nil, err
		}

		if d, ok := t.(json.Delim); ok {
			switch d {
			case '[':
				token = t
				decoder = newDecoder

			default:
				// doing this again as it json file only has an object and it is not an array
				name := f.Name()
				err = f.File.Close()
				if err != nil {
					f.logger.Errorf("failed to close JSON file for reading as object %v", err)

					return nil, err
				}
				newFile, err := os.Open(name)
				if err != nil {
					f.logger.Errorf("failed to open JSON file for reading as object %v", err)

					return nil, err
				}

				decoder = json.NewDecoder(newFile)
			}
		}

		jd := jsonReader{decoder: decoder, token: token}

		return &jd, nil
	}

	return &textCSVReader{
		scanner: bufio.NewScanner(f.File)}, nil
}

func (j jsonReader) Next() bool {
	return j.decoder.More()
}

func (j jsonReader) Scan(i interface{}) error {
	return j.decoder.Decode(&i)
}

func (f textCSVReader) Next() bool {
	return f.scanner.Scan()
}

func (f textCSVReader) Scan(i interface{}) error {
	switch target := i.(type) {
	case *string:
		*target = f.scanner.Text()
		return nil
	default:
		return errors.New("scan destination must be a string pointer")
	}
}
