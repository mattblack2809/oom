package main

import (
  //"fmt"
  "matt/oom"
  "flag"
  "sort"
  "time"
  "log"
  "os"
  "fmt"
)

type PlayerOOM struct {
  Name string
  Rank int
  OOMPoints int
  NumCompetitions int
  PlayerByComp map[string]oom.PlayerResult // map keyed on comp key
}

type OOM struct {
  Year int
  Competitions []oom.Competition
  RankedPlayers []string // first to last in results
  OOMResults map[string]PlayerOOM // map keyed by plaer name
}

var theOOM OOM // don't need more than 1
var flagDetail *bool

func main() {
  flagAll := flag.Bool("all", false, "true for all comps")
  flagYear := flag.Int("year", 0, "default to current year")
  flagDetail = flag.Bool("detail", false, "set to true to output player rank and result additional to oom points")
  flag.Parse()

  t := time.Now()
  if(*flagYear == 0) {
    theOOM.Year = t.Year()
  } else {
    theOOM.Year = *flagYear
  }

  if(*flagAll == true) {
    theOOM.Competitions = oom.FetchAllCompDesc(theOOM.Year)
  } else {
    theOOM.Competitions = oom.FetchCompDescriptions(theOOM.Year, "oom.conf")
  }
  slots := make(chan int, 10) // max concurrent calls to oom.Load
  // use another channel to wait for all go routines to complete
  completed := make(chan int, len(theOOM.Competitions))
  // don't iterate - we want to pass the address of each Competition
  var concurrent int
  for i:=0; i < len(theOOM.Competitions); i++ {
    slots <- 1 // get a slot
    go func(comp *oom.Competition) {
      concurrent++; //fmt.Println("Increment Concurrent ", concurrent)
      oom.Load(comp)
      completed <- 1
      <- slots // release slot
      concurrent--; //fmt.Println("Decrement Concurrent ", concurrent)
    }(&theOOM.Competitions[i])
  }
  for range theOOM.Competitions { // wait for go routines to complete
    //fmt.Println("Concurrent is ", concurrent)
    <- completed
  }
  populateOOMWithCompetitions()
  calculateOOMRank()
  printOOM()
}

// Transpose the data from the []Competitions in to the map keyed by player
func populateOOMWithCompetitions() {
  theOOM.OOMResults = make(map[string]PlayerOOM)
  for i := range theOOM.Competitions {
    comp := &theOOM.Competitions[i] // Competitions is a slice
    for name, result := range comp.Results {
      // if player not seen before initialise their PlayerOOM entry
      playerOOM, ok := theOOM.OOMResults[name] // can't take address - why?
      if !ok {
        playerOOM.Name = name
        playerOOM.PlayerByComp = make(map[string]oom.PlayerResult)
      }
      playerOOM.PlayerByComp[comp.Key] = oom.PlayerResult{
        Name: name,
        OOMPoints: result.OOMPoints,
        Rank: result.Rank,
        Result: result.Result,
      }
      playerOOM.OOMPoints += result.OOMPoints
      playerOOM.NumCompetitions ++
      theOOM.OOMResults[name] = playerOOM
    }
  }
}

type rankElem struct{name string; oomPoints int}
type rankSlice []rankElem
func (l rankSlice) Len() int {return len(l)}
func (l rankSlice) Less(i int, j int) bool {return l[i].oomPoints < l[j].oomPoints}
func (l rankSlice) Swap(i int, j int) {l[i], l[j] = l[j], l[i]}
func calculateOOMRank() {
  var rs rankSlice
  for name, oomRes := range theOOM.OOMResults {
    rs = append(rs, rankElem{name, oomRes.OOMPoints})
  }
  sort.Sort(sort.Reverse(rs))
  var rankedPlayers []string
  rank:=1
  for n, p := range rs {
    rankedPlayers = append(rankedPlayers, p.name)
    pOOM := theOOM.OOMResults[p.name]
    pOOM.Rank = rank + n // can't directly assign to struct field within map
    theOOM.OOMResults[p.name] = pOOM
  }
  theOOM.RankedPlayers = rankedPlayers
}

func printOOM() {
  f, err := os.Create("out.csv")
  if err != nil {log.Fatal(err)}
  defer f.Close()
  fmt.Fprintf(f, "Year %d\n", theOOM.Year)
  fmt.Fprint(f, ",,,,")
  for _, comp := range theOOM.Competitions {
    fmt.Fprint(f, comp.Key, ",")
  }
  fmt.Fprint(f, "\n")
  fmt.Fprint(f, ",,,,")
  for _, comp := range theOOM.Competitions {
    fmt.Fprint(f, comp.Date, ",")
  }
  fmt.Fprint(f, "\n")
  fmt.Fprintf(f, "rank, name, oomPts, #Comp,")
  for _, comp := range theOOM.Competitions {
    fmt.Fprint(f, comp.Name, ",")
  }
  fmt.Fprint(f, "\n")
  for _, player := range theOOM.RankedPlayers {
    fmt.Fprint(f, theOOM.OOMResults[player].Rank, ",",
      theOOM.OOMResults[player].Name, ",",
      theOOM.OOMResults[player].OOMPoints, ",",
      theOOM.OOMResults[player].NumCompetitions)
    for _, comp := range theOOM.Competitions {
      playerResult, ok := theOOM.OOMResults[player].PlayerByComp[comp.Key]
      if ok {
        fmt.Fprint(f, ",", formatPlayerResult(playerResult))
      } else {
        fmt.Fprint(f, ",")
      }
    }
    fmt.Fprint(f, "\n")
  }
}

func formatPlayerResult(p oom.PlayerResult) string {
  if *flagDetail == false {
    return fmt.Sprintf("%d", p.OOMPoints)
  }
  nth := "th"
	s :=fmt.Sprintf("%d", p.Rank)
	l := s[len(s) - 1:]
  if l == "1" {nth = "st"}
  if l == "2" {nth = "nd"}
  if l == "3" {nth = "rd"}
  if p.Rank > 10 && p.Rank < 20 {
    nth = "th"
  }
  return fmt.Sprintf("%d (%d%s %s)",
    p.OOMPoints,
    p.Rank, nth,
    p.Result)
}
