<p align="center">
  <img src="https://upper.io/db.v3/images/gopher.svg" width="256" />
</p>

# upper.io/db.v3 [![Build Status](https://travis-ci.org/upper/db.svg?branch=master)](https://travis-ci.org/upper/db) [![GoDoc](https://godoc.org/upper.io/db.v3?status.svg)](https://godoc.org/upper.io/db.v3)

The `upper.io/db.v3` package for [Go][2] is a productive data access layer for
Go that provides a common interface to work with different data sources such as
[PostgreSQL](https://upper.io/db.v3/postgresql),
[MySQL](https://upper.io/db.v3/mysql), [SQLite](https://upper.io/db.v3/sqlite),
[QL](https://upper.io/db.v3/ql) and [MongoDB](https://upper.io/db.v3/mongodb).

```
go get upper.io/db.v3
```

## The tour

![screen shot 2017-05-01 at 19 23 22](https://cloud.githubusercontent.com/assets/385670/25599675/b6fe9fea-2ea3-11e7-9f76-002931dfbbc1.png)

Take the [tour](https://tour.upper.io) to see real live examples in your
browser.

## Live demos

You can run the following example on our [playground](https://demo.upper.io):

```go
package main

import (
	"log"

	"upper.io/db.v3/postgresql"
)

var settings = postgresql.ConnectionURL{
	Host:     "demo.upper.io",
	Database: "booktown",
	User:     "demouser",
	Password: "demop4ss",
}

type Book struct {
	ID        int    `db:"id"`
	Title     string `db:"title"`
	AuthorID  int    `db:"author_id"`
	SubjectID int    `db:"subject_id"`
}

func main() {
	sess, err := postgresql.Open(settings)
	if err != nil {
		log.Fatalf("db.Open(): %q\n", err)
	}
	defer sess.Close()

	var books []Book
	err = sess.Collection("books").Find().All(&books)
	if err != nil {
		log.Fatalf("Find(): %q\n", err)
	}

	for i, book := range books {
		log.Printf("Book %d: %#v\n", i, book)
	}
}
```

Or you can also run it locally from the `_examples` directory:

```
go run _examples/booktown-books/main.go
2016/08/10 08:42:48 "The Shining" (ID: 7808)
2016/08/10 08:42:48 "Dune" (ID: 4513)
2016/08/10 08:42:48 "2001: A Space Odyssey" (ID: 4267)
2016/08/10 08:42:48 "The Cat in the Hat" (ID: 1608)
2016/08/10 08:42:48 "Bartholomew and the Oobleck" (ID: 1590)
2016/08/10 08:42:48 "Franklin in the Dark" (ID: 25908)
2016/08/10 08:42:48 "Goodnight Moon" (ID: 1501)
2016/08/10 08:42:48 "Little Women" (ID: 190)
2016/08/10 08:42:48 "The Velveteen Rabbit" (ID: 1234)
2016/08/10 08:42:48 "Dynamic Anatomy" (ID: 2038)
2016/08/10 08:42:48 "The Tell-Tale Heart" (ID: 156)
2016/08/10 08:42:48 "Programming Python" (ID: 41473)
2016/08/10 08:42:48 "Learning Python" (ID: 41477)
2016/08/10 08:42:48 "Perl Cookbook" (ID: 41478)
2016/08/10 08:42:48 "Practical PostgreSQL" (ID: 41472)
```

## Documentation for users

This is the source code repository, check out our [release
notes](https://github.com/upper/db/releases/tag/v3.0.0) and see examples and
documentation at [upper.io/db.v3][1].


## Changelog

See [CHANGELOG.md](https://github.com/upper/db/blob/master/CHANGELOG.md).

## License

This project is licensed under the terms of the **MIT License**.

> Copyright (c) 2012-present The upper.io/db authors. All rights reserved.
>
> Permission is hereby granted, free of charge, to any person obtaining
> a copy of this software and associated documentation files (the
> "Software"), to deal in the Software without restriction, including
> without limitation the rights to use, copy, modify, merge, publish,
> distribute, sublicense, and/or sell copies of the Software, and to
> permit persons to whom the Software is furnished to do so, subject to
> the following conditions:
>
> The above copyright notice and this permission notice shall be
> included in all copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
> EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
> MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
> NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
> LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
> OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
> WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

## Authors and contributors

* José Carlos Nieto <<jose.carlos@menteslibres.net>>
* Peter Kieltyka <<peter@pressly.com>>
* Maciej Lisiewski <<maciej.lisiewski@gmail.com>>
* Max Hawkins <<maxhawkins@google.com>>
* Paul Xue <<paul.xue@pressly.com>>
* Kevin Darlington <<kdarlington@gmail.com>>
* Lars Buitinck <<l.buitinck@esciencecenter.nl>>
* icattlecoder <<icattlecoder@gmail.com>>
* Aaron <<aaron.l.france@gmail.com>>
* Hiram J. Pérez <<worg@linuxmail.org>>
* Julien Schmidt <<github@julienschmidt.com>>
* Max Hawkins <<maxhawkins@gmail.com>>
* Piotr "Orange" Zduniak <<piotr@zduniak.net>>
* achun <<achun.shx@qq.com>>
* rjmcguire <<rjmcguire@gmail.com>>
* wei2912 <<wei2912_support@hotmail.com>>

[1]: https://upper.io/db.v3
[2]: http://golang.org
