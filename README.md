# Godocx-templates

Template-based docx report creation. ([See the blog post](http://guigrpa.github.io/2017/01/01/word-docs-the-relay-way/)).

HEAVILY inspired (aka copy/pasted) from [docx-templates](https://github.com/guigrpa/docx-templates) 🙏

```sh
go get github.com/ArFnds/godocx-template
```

## Why?

* **Write documents naturally using Word**, just adding some commands where needed for dynamic contents

## Features
* **Insert the data** in your document (`INS`, `=` or just *nothing*)
* **Embed images and HTML** (`IMAGE`, `HTML`). Dynamic images can be great for on-the-fly QR codes, downloading photos straight to your reports, charts… even maps!
* Add **loops** with `FOR`/`END-FOR` commands, with support for table rows, nested loops
* Include contents conditionally, IF a certain code expression is truthy (`IF`/`END-IF`)
* Define custom **aliases** for some commands (`ALIAS`) — useful for writing table templates!
* Plenty of **examples** in this repo
* **Embed hyperlinks** (`LINK`).

### Not yet supported

Contributions are welcome!

# Table of contents

- [Godocx-templates](#godocx-templates)
	- [Why?](#why)
	- [Features](#features)
		- [Not yet supported](#not-yet-supported)
- [Table of contents](#table-of-contents)
- [Installation](#installation)
- [Usage](#usage)
- [Writing templates](#writing-templates)
	- [Custom command delimiters](#custom-command-delimiters)
	- [Supported commands](#supported-commands)
		- [Insert data with the `INS` command ( or using `=`, or nothing at all)](#insert-data-with-the-ins-command--or-using--or-nothing-at-all)
		- [`LINK`](#link)
		- [`HTML`](#html)
		- [`IMAGE`](#image)
		- [`FOR` and `END-FOR`](#for-and-end-for)
		- [`IF` and `END-IF`](#if-and-end-if)
		- [`ALIAS` (and alias resolution with `*`)](#alias-and-alias-resolution-with-)
	- [Inserting literal XML](#inserting-literal-xml)
- [License (MIT)](#license-mit)

# Installation

```
$ go get github.com/ArFnds/godocx-template
```

# Usage

Here is a simple example, with report data injected directly as an object:

```go
import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"

	. "github.com/ArFnds/godocx-template"
)

func main() {
   var data = ReportData{
		"dateOfDay":         time.Now().Local().Format("02/01/2006"),
		"acceptDate":        time.Now().Local().Format("02/01/2006"),
		"company":           "The company",
		"people": []any{
			map[string]any{"name": "John", "lastname": "Doe"},
			map[string]any{"name": "Barn", "lastname": "Simson"},
		},
   }

   options := CreateReportOptions{
        // mandatory
		LiteralXmlDelimiter: "||",
		// optionals
		ProcessLineBreaks: true,
   }

   outBuf, err := CreateReport("mytemplate.docx", &data, options)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("outdoc.docx", outBuf, 0644)
	if err != nil {
		panic(err)
	}
}

```


# Writing templates

Create a word file, and write your template inside it.

```
dateOfDay: +++dateOfDay+++  acceptDate: +++acceptDate+++
company: +++company+++

+++FOR person IN people+++
  person:  +++INS $person.firstname+++  +++INS $person.lastname+++
+++END-FOR person+++
```

## Custom command delimiters
You can use different **left/right command delimiters** by passing an object to `CmdDelimiter`:


```go
options := CreateReportOptions{
	LiteralXmlDelimiter: "||",
	CmdDelimiter: &Delimiters{
		Open:  "{",
		Close: "}",
	},
}
```

This allows much cleaner-looking templates!

Then you can add commands in your template like this: `{foo}`, `{project.name}`, `{FOR ...}`.


## Supported commands
Currently supported commands are defined below.

### Insert data with the `INS` command ( or using `=`, or nothing at all)

Inserts the result of a given code snippet as follows.

Using code like this:
```go
import (
	"fmt"
	"log/slog"
	"os"
	"time"

	. "github.com/ArFnds/godocx-template"
)

func main() {
   var data = ReportData{
		"name":    "John",
		"surname": "Appleseed",
   }

   options := CreateReportOptions{
		LiteralXmlDelimiter: "||",
   }

   outBuf, err := CreateReport("mytemplate.docx", &data, options)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("outdoc.docx", outBuf, 0644)
	if err != nil {
		panic(err)
	}
}
```
And a template like this:

```
+++name+++ +++surname+++
```

Will produce a result docx file that looks like this:

```
John Appleseed
```

Alternatively, you can use the more explicit `INS` (insert) command syntax.
```
+++INS name+++ +++INS surname+++
```

You can also use `=` as shorthand notation instead of `INS`:

```
+++= name+++ +++= surname+++
```

Even shorter (and with custom `CmdDelimiter: &Delimiters{Open: "{", Close: "}"}`):

```
{name} {surname}
```

### `LINK`

Includes a hyperlink from a `map[string]any` with a `url` and `label` key,  or `*LinkPars`:

```go
data := ReportData {
	"projectLink": &LinkPars {
		Url: "https://theproject.url",
		Label: "The label"
	}
}
```


```
+++LINK projectLink+++
```

If the `label` is not specified, the URL is used as a label.

### `HTML`

Takes the HTML resulting from evaluating a code snippet and converts it to Word contents.

**Important:** This uses [altchunk](https://blogs.msdn.microsoft.com/ericwhite/2008/10/26/how-to-use-altchunk-for-document-assembly/), which is only supported in Microsoft Word, and not in e.g. LibreOffice or Google Docs.

```
+++HTML `
<meta charset="UTF-8">
<body>
  <h1>${$film.title}</h1>
  <h3>${$film.releaseDate.slice(0, 4)}</h3>
  <p>
    <strong style="color: red;">This paragraph should be red and strong</strong>
  </p>
</body>
`+++
```


### `IMAGE`

The value should be an _ImagePars_, containing:

* `width`: desired width of the image on the page _in cm_. Note that the aspect ratio should match that of the input image to avoid stretching.
* `height` desired height of the image on the page _in cm_.
* `data`: an ByteArray with the image data
* `extension`: one of `'.png'`, `'.gif'`, `'.jpg'`, `'.jpeg'`, `'.svg'`.
* `thumbnail` _[optional]_: when injecting an SVG image, a fallback non-SVG (png/jpg/gif, etc.) image can be provided. This thumbnail is used when SVG images are not supported (e.g. older versions of Word) or when the document is previewed by e.g. Windows Explorer. See usage example below.
* `alt` _[optional]_: optional alt text.
* `rotation` _[optional]_: optional rotation in degrees, with positive angles moving clockwise.
* `caption` _[optional]_: optional caption displayed below the image

In the .docx template:
```
+++IMAGE imageKey+++
```

Note that you can center the image by centering the IMAGE command in the template.

In the `ReportData`:
```go
data := ReportData {
  "imageKey": &ImagePars{
			Width:     16.88,
			Height:    23.74,
			Data:      imageByteArray,
			Extension: ".png",
		},
}
```

### `FOR` and `END-FOR`

Loop over a group of elements (can only iterate over Array).
```
+++FOR person IN peopleArray+++
+++INS $person.name+++ (since +++INS $person.since+++)
+++END-FOR person+++
```

Note that inside the loop, the variable relative to the current element being processed must be prefixed with `$`.

It is possible to get the current element index of the inner-most loop with the variable `$idx`, starting from `0`. For example:
```
+++FOR company IN companies+++
Company (+++$idx+++): +++INS $company.name+++
Executives:
+++FOR executive IN $company.executives+++
-	+++$idx+++ +++$executive+++
+++END-FOR executive+++
+++END-FOR company+++
```

`FOR` loops also work over table rows:

```
----------------------------------------------------------
| Name                         | Since                   |
----------------------------------------------------------
| +++FOR person IN             |                         |
| project.people+++            |                         |
----------------------------------------------------------
| +++INS $person.name+++       | +++INS $person.since+++ |
----------------------------------------------------------
| +++END-FOR person+++         |                         |
----------------------------------------------------------
```

And let you dynamically generate columns:

```
+-------------------------------+--------------------+------------------------+
| +++ FOR row IN rows+++        |                    |                        |
+===============================+====================+========================+
| +++ FOR column IN columns +++ | +++INS $row+++     | +++ END-FOR column +++ |
|                               |                    |                        |
|                               | Some cell content  |                        |
|                               |                    |                        |
|                               | +++INS $column+++  |                        |
+-------------------------------+--------------------+------------------------+
| +++ END-FOR row+++            |                    |                        |
+-------------------------------+--------------------+------------------------+
```

Finally, you can nest loops (this example assumes a different data set):

```
+++FOR company IN companies+++
+++INS $company.name+++
+++FOR person IN $company.people+++
* +++INS $person.firstName+++
+++FOR project IN $person.projects+++
    - +++INS $project.name+++
+++END-FOR project+++
+++END-FOR person+++

+++END-FOR company+++
```
### `IF` and `END-IF`

Include contents conditionally (support: ==, !=, >=, <=, >, <):

```
+++IF name == 'John'+++
 Name is John
+++END-IF+++
```

The `IF` command is implemented as a `FOR` command with 1 or 0 iterations, depending on the expression value.

### `ALIAS` (and alias resolution with `*`)

Define a name for a complete command (especially useful for formatting tables):

```
+++ALIAS name INS $person.name+++
+++ALIAS since INS $person.since+++

----------------------------------------------------------
| Name                         | Since                   |
----------------------------------------------------------
| +++FOR person IN             |                         |
| project.people+++            |                         |
----------------------------------------------------------
| +++*name+++                  | +++*since+++            |
----------------------------------------------------------
| +++END-FOR person+++         |                         |
----------------------------------------------------------
```

## Inserting literal XML
You can also directly insert Office Open XML markup into the document using the `literalXmlDelimiter`, which is by default set to `||`.

E.g. if you have a template like this:

```
+++INS text+++
```

```go
data := ReportData{ "text": "foo||<w:br/>||bar" }
```

See http://officeopenxml.com/anatomyofOOXML.php for a good reference of the internal XML structure of a docx file.

# License (MIT)

This Project is licensed under the MIT License. See [LICENSE](LICENSE) for more information.
