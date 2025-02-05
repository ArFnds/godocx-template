package godocx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func createTestDocx(content []byte, filename string) error {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add files to the archive.
	files := map[string][]byte{
		"[Content_Types].xml": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
			<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
			<Default Extension="xml" ContentType="application/xml"/>
			<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
		</Types>`),
		"_rels/.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
			<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
		</Relationships>`),
		"word/document.xml": content,
		"word/_rels/document.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
		</Relationships>`),
	}

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(content)
		if err != nil {
			return err
		}
	}

	// Make sure to check the error on Close.
	err := w.Close()
	if err != nil {
		return err
	}

	return os.WriteFile(filename, buf.Bytes(), 0644)
}
func verifyDocxContent(t *testing.T, filename string, verifyFn func([]byte) error) {
	// Open the docx file
	outputZip, err := zip.OpenReader(filename)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer outputZip.Close()

	// Find and read document.xml content
	var documentXml []byte
	for _, f := range outputZip.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open document.xml: %v", err)
			}
			documentXml, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("Failed to read document.xml: %v", err)
			}
			break
		}
	}

	// Run the verification function
	if err := verifyFn(documentXml); err != nil {
		t.Error(err)
	}
}

func TestCreateReport(t *testing.T) {
	// Test basic data processing
	t.Run("basic data processing", func(t *testing.T) {
		data := ReportData{
			"name":    "John",
			"surname": "Doe",
		}

		// Create template files for testing
		templateContent := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:body>
				<w:p>
					<w:r>
						<w:t>+++name+++ +++surname+++</w:t>
					</w:r>
				</w:p>
			</w:body>
		</w:document>`)
		err := createTestDocx(templateContent, "test_template.docx")
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
		defer os.Remove("test_template.docx")

		// Run test
		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
		}

		outBuf, err := CreateReport("test_template.docx", &data, options)
		if err != nil {
			t.Fatalf("CreateReport failed: %v", err)
		}

		// Save the output file
		os.WriteFile("test_output.docx", outBuf, 0644)
		defer os.Remove("test_output.docx")

		// Verify the generated file content
		verifyDocxContent(t, "test_output.docx", func(documentXml []byte) error {
			expectedValues := []string{"John", "Doe"}
			for _, val := range expectedValues {
				if !bytes.Contains(documentXml, []byte(val)) {
					return fmt.Errorf("Generated document does not contain expected value: %s", val)
				}
			}
			return nil
		})
	})

	// Test image processing
	t.Run("image processing", func(t *testing.T) {
		imageData := []byte{
			137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 50, 0, 0, 0, 50, 8, 2, 0, 0, 0, 145, 93, 31, 230, 0, 0, 0, 30, 73, 68, 65, 84, 120, 156, 237, 193, 49, 1, 0, 0, 0, 194, 160, 245, 79, 109, 8, 95, 160, 0, 0, 0, 0, 0, 0, 248, 13, 29, 126, 0, 1, 10, 82, 239, 54, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130,
		}
		data := ReportData{
			"img": &ImagePars{
				Width:     5,
				Height:    5,
				Data:      imageData,
				Extension: ".png",
			},
		}

		// Create template files for testing
		templateContent := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:body>
				<w:p>
					<w:r>
						<w:t>+++IMAGE img+++</w:t>
					</w:r>
				</w:p>
			</w:body>
		</w:document>`)
		err := createTestDocx(templateContent, "test_template_image.docx")
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
		defer os.Remove("test_template_image.docx")

		// Run test
		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
		}

		outBuf, err := CreateReport("test_template_image.docx", &data, options)
		if err != nil {
			t.Fatalf("CreateReport failed: %v", err)
		}

		// Save file
		os.WriteFile("test_output_image.docx", outBuf, 0644)
		defer os.Remove("test_output_image.docx")

		// Verify the generated file content for image
		verifyDocxContent(t, "test_output_image.docx", func(documentXml []byte) error {
			// No specific content verification needed for image test
			return nil
		})

		// Open the docx file for additional verifications
		outputZip, err := zip.OpenReader("test_output_image.docx")
		if err != nil {
			t.Fatalf("Failed to open output file: %v", err)
		}
		defer outputZip.Close()

		// Verify media folder exists and contains the image
		imageFound := false
		for _, f := range outputZip.File {
			if strings.HasPrefix(f.Name, "word/media/") && strings.HasSuffix(f.Name, ".png") {
				imageFound = true
				break
			}
		}
		if !imageFound {
			t.Error("No image file found in the generated document")
		}

		// Verify Content_Types.xml contains image content type
		var contentTypesXml []byte
		for _, f := range outputZip.File {
			if f.Name == "[Content_Types].xml" {
				rc, err := f.Open()
				if err != nil {
					t.Fatalf("Failed to open [Content_Types].xml: %v", err)
				}
				contentTypesXml, err = io.ReadAll(rc)
				rc.Close()
				if err != nil {
					t.Fatalf("Failed to read [Content_Types].xml: %v", err)
				}
				break
			}
		}

		if !bytes.Contains(contentTypesXml, []byte("image/png")) {
			t.Error("[Content_Types].xml does not contain image/png content type")
		}
	})

	// Test error handling
	t.Run("error handling - invalid template", func(t *testing.T) {
		data := ReportData{}
		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
		}

		_, err := CreateReport("non_existent_template.docx", &data, options)
		if err == nil {
			t.Fatal("Expected error for non-existent template, but got none")
		}
	})

	// Test custom delimiters
	t.Run("custom delimiters", func(t *testing.T) {
		data := ReportData{
			"name": "John",
		}

		templateContent := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:body>
				<w:p>
					<w:r>
						<w:t>{name}</w:t>
					</w:r>
				</w:p>
			</w:body>
		</w:document>`)
		err := createTestDocx(templateContent, "test_template_custom.docx")
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
		defer os.Remove("test_template_custom.docx")

		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
			CmdDelimiter: &Delimiters{
				Open:  "{",
				Close: "}",
			},
		}

		outBuf, err := CreateReport("test_template_custom.docx", &data, options)
		if err != nil {
			t.Fatalf("CreateReport failed: %v", err)
		}

		// Save the output file
		os.WriteFile("test_output_custom.docx", outBuf, 0644)
		defer os.Remove("test_output_custom.docx")

		// Verify the generated file content
		verifyDocxContent(t, "test_output_custom.docx", func(documentXml []byte) error {
			if !bytes.Contains(documentXml, []byte("John")) {
				return fmt.Errorf("Generated document does not contain expected value: John")
			}
			return nil
		})
	})

	// Test FOR loop processing
	t.Run("for loop processing", func(t *testing.T) {
		data := ReportData{
			"items": []any{
				map[string]any{"name": "Item 1", "value": 100},
				map[string]any{"name": "Item 2", "value": 200},
				map[string]any{"name": "Item 3", "value": 300},
			},
			"categories": []any{
				map[string]any{
					"name": "Category 1",
					"subitems": []any{
						"Sub 1.1",
						"Sub 1.2",
					},
				},
				map[string]any{
					"name": "Category 2",
					"subitems": []any{
						"Sub 2.1",
						"Sub 2.2",
					},
				},
			},
		}

		// Create template files for testing
		templateContent := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:body>
				<w:p>
					<w:r>
						<w:t>Simple Loop:</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++FOR item IN items+++</w:t>
						<w:t>Name: +++INS $item.name+++ Value: +++INS $item.value+++</w:t>
						<w:t>+++END-FOR item+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>Nested Loop:</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++FOR category IN categories+++</w:t>
						<w:t>Category: +++INS $category.name+++</w:t>
						<w:t>+++FOR subitem IN $category.subitems+++</w:t>
						<w:t>  - +++INS $subitem+++</w:t>
						<w:t>+++END-FOR subitem+++</w:t>
						<w:t>+++END-FOR category+++</w:t>
					</w:r>
				</w:p>
			</w:body>
		</w:document>`)
		err := createTestDocx(templateContent, "test_template_loop.docx")
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
		defer os.Remove("test_template_loop.docx")

		// Run test
		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
		}

		outBuf, err := CreateReport("test_template_loop.docx", &data, options)
		if err != nil {
			t.Fatalf("CreateReport failed: %v", err)
		}

		// Save the output file
		os.WriteFile("test_output_loop.docx", outBuf, 0644)
		defer os.Remove("test_output_loop.docx")

		// Verify the generated file content
		verifyDocxContent(t, "test_output_loop.docx", func(documentXml []byte) error {
			expectedValues := []string{
				"Item 1", "100",
				"Item 2", "200",
				"Item 3", "300",
				"Category 1", "Sub 1.1", "Sub 1.2",
				"Category 2", "Sub 2.1", "Sub 2.2",
			}
			for _, val := range expectedValues {
				if !bytes.Contains(documentXml, []byte(val)) {
					return fmt.Errorf("Generated document does not contain expected value: %s", val)
				}
			}
			return nil
		})
	})

	// Test IF condition processing
	t.Run("if condition processing", func(t *testing.T) {
		data := ReportData{
			"isActive":   true,
			"isInactive": false,
			"name":       "John",
			"age":        25,
			"score":      85,
		}

		// Create template files for testing
		templateContent := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:body>
				<w:p>
					<w:r>
						<w:t>Boolean Test:</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF isActive+++</w:t>
						<w:t>Active user</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF name == 'John'+++</w:t>
						<w:t>Name is John</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF name != 'Sean'+++</w:t>
						<w:t>Name is not Sean</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF age > 18+++</w:t>
						<w:t>Adult</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF score >= 80+++</w:t>
						<w:t>High Score</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF isInactive+++</w:t>
						<w:t>Inactive user</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++IF name == 'Jane'+++</w:t>
						<w:t>Name is Jane</w:t>
						<w:t>+++END-IF+++</w:t>
					</w:r>
				</w:p>
			</w:body>
		</w:document>`)
		err := createTestDocx(templateContent, "test_template_if.docx")
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
		defer os.Remove("test_template_if.docx")

		// Run test
		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
		}

		outBuf, err := CreateReport("test_template_if.docx", &data, options)
		if err != nil {
			t.Fatalf("CreateReport failed: %v", err)
		}

		// Save the output file
		os.WriteFile("test_output_if.docx", outBuf, 0644)
		defer os.Remove("test_output_if.docx")

		// Verify the generated file content
		verifyDocxContent(t, "test_output_if.docx", func(documentXml []byte) error {
			// Should appear in document
			expectedPresent := []string{
				"Active user",
				"Name is John",
				"Adult",
				"High Score",
				"Name is not Sean",
			}
			for _, val := range expectedPresent {
				if !bytes.Contains(documentXml, []byte(val)) {
					return fmt.Errorf("Generated document does not contain expected value: %s", val)
				}
			}

			// Should not appear in document
			expectedAbsent := []string{
				"Inactive user",
				"Name is Jane",
			}
			for _, val := range expectedAbsent {
				if bytes.Contains(documentXml, []byte(val)) {
					return fmt.Errorf("Generated document contains unexpected value: %s", val)
				}
			}
			return nil
		})
	})

	// Test LINK processing
	t.Run("link processing", func(t *testing.T) {
		data := ReportData{
			"projectLink": &LinkPars{
				Url:   "https://github.com/project",
				Label: "Project Repository",
			},
			"simpleLink": map[string]any{
				"url": "https://example.com",
			},
		}

		// Create template files for testing
		templateContent := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:body>
				<w:p>
					<w:r>
						<w:t>+++LINK projectLink+++</w:t>
					</w:r>
				</w:p>
				<w:p>
					<w:r>
						<w:t>+++LINK simpleLink+++</w:t>
					</w:r>
				</w:p>
			</w:body>
		</w:document>`)
		err := createTestDocx(templateContent, "test_template_link.docx")
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
		defer os.Remove("test_template_link.docx")

		// Run test
		options := CreateReportOptions{
			LiteralXmlDelimiter: "||",
		}

		outBuf, err := CreateReport("test_template_link.docx", &data, options)
		if err != nil {
			t.Fatalf("CreateReport failed: %v", err)
		}

		// Save the output file
		os.WriteFile("test_output_link.docx", outBuf, 0644)
		defer os.Remove("test_output_link.docx")

		// Verify the generated file content
		verifyDocxContent(t, "test_output_link.docx", func(documentXml []byte) error {
			expectedValues := []string{
				"Project Repository",
				"https://example.com",
			}
			for _, val := range expectedValues {
				if !bytes.Contains(documentXml, []byte(val)) {
					return fmt.Errorf("Generated document does not contain expected value: %s", val)
				}
			}
			return nil
		})
	})

}
