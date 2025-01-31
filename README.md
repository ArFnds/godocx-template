# Godocx-templates 

Template-based docx report creation. ([See the blog post](http://guigrpa.github.io/2017/01/01/word-docs-the-relay-way/)).


## Why?

* **Write documents naturally using Word**, just adding some commands where needed for dynamic contents

## Features 
* **Insert the data** in your document (`INS`, `=` or just *nothing*)
* **Embed images and HTML** (`IMAGE`, `HTML`). Dynamic images can be great for on-the-fly QR codes, downloading photos straight to your reports, charts… even maps!
* Add **loops** with `FOR`/`END-FOR` commands, with support for table rows, nested loops, and JavaScript processing of elements (filter, sort, etc)


### Not yet supported
- [ ] **Embed hyperlinks and even HTML dynamically** (`LINK`, `HTML`).
- [ ] Include contents **conditionally**, `IF` a certain JavaScript expression is truthy
- [ ] Define custom **aliases** for some commands (`ALIAS`) — useful for writing table templates!
- [ ] Include **literal XML**
- [ ] Written in TypeScript, so ships with type definitions.
- [ ] Plenty of **examples** in this repo (with Node, Webpack and Browserify)
- [ ] Supports `.docm` templates in addition to regular `.docx` files.

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
		- [`HTML`](#html)
		- [`IMAGE`](#image)
		- [`FOR` and `END-FOR`](#for-and-end-for)
	- [Inserting literal XML](#inserting-literal-xml)
- [Error handling](#error-handling)
- [Performance \& security](#performance--security)
- [License (MIT)](#license-mit)

# Installation

```
$ go get github.com/ArFnds/godocx-template/pkg/report
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

	. "github.com/ArFnds/godocx-template/pkg/report"
)

func main() {
   var data = ReportData{
		"dateOfDay":         time.Now().Local().Format("02/01/2006"),
		"acceptDate":        time.Now().Local().Format("02/01/2006"),
		"company":           "The company",
      "persons": []map[string]any{
			{"firstname": "John", "lastname": "Doe"},
			{"firstname": "Barn", "lastname": "Simson"},
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

**TODO**

## Custom command delimiters
You can use different **left/right command delimiters** by passing an object to `CmdDelimiter`:


**TODO**

This allows much cleaner-looking templates!

Then you can add commands in your template like this: `{foo}`, `{project.name}`, `{FOR ...}`.


## Supported commands
Currently supported commands are defined below.

### Insert data with the `INS` command ( or using `=`, or nothing at all)

Inserts the result of a given JavaScript snippet as follows.

Using code like this:
```go
import (
	"fmt"
	"log/slog"
	"os"
	"time"

	. "github.com/ArFnds/godocx-template/pkg/report"
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

Even shorter (and with custom `cmdDelimiter: ['{', '}']`):

```
{name} {surname}
```
### `HTML`

Takes the HTML resulting from evaluating a JavaScript snippet and converts it to Word contents.

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

**TODO**

The value should be an _ImagePars_, containing:

* `width`: desired width of the image on the page _in cm_. Note that the aspect ratio should match that of the input image to avoid stretching.
* `height` desired height of the image on the page _in cm_.
* `data`: either an ArrayBuffer or a base64 string with the image data
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

Loop over a group of elements (resulting from the evaluation of a JavaScript expression):

```
+++FOR person IN personsArray+++
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

# Error handling

**TODO**

# Performance & security



# License (MIT)

This Project is licensed under the MIT License. See [LICENSE](LICENSE) for more information.
