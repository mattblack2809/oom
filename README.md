# oom
A rework in git with unit tests and gofmt and golint
Clone to a directory structure that fits the Go language tree:
main.go import matt/oom so the tree under $GOPATH should have

-$GOPATH
>-src
>>-matt <<< make a matt directory
>>>-oom <<< make an oom directory and clone to here

The main.go file is in a further sub-directory $GOPATH/src/matt/oom/oom/main.go

Note the MS spreadsheet uses a 2nd tab that links to out.csv.  Due to MS crapness
the path saved in the Excel file is absolute so you will need to edit the
data source...
