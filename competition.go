package oom

// competition.go handles reading and parsing both the list of competitions
// of interest, and each comptition.  It reads information from the cgc
// website if required - and caches it locally in files so that subsequent
// runs can be completed 'off-line'
//
// The set of files consulted and generated as follows:
// - 5462.txt caches results of competition with key of 5462 in human readable
//   and editable form.  Results of match play following certain stroke play
//   compeitions will be used to manually update the relevant result file.
// - all_comps.dat caches (in binary form) the page at URL:
//   "http://www.colchestergolfclub.com/competition.php?showall=1
//      &time=&show=&year=%d", year
//   noting that this file should be manually purged to start working on
//   a different year - that is a bug TODO to fix
// - fname param to FetchCompDescriptions names a file containing the Key and
//   optionally full URL of each competition of interest.
//   As a minimum each line contains "cystic fibrosis, ?compid=1239" where
//   the ?compid=1239 contains the (vital) competition key and may be
//   part of a full URL as copied from the website

import (
	"bufio"
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"fmt"
	"strconv"
	"strings"
	"net/url"
)

// PlayerResult represents how a single player scored in a single competition
type PlayerResult struct {
	Name      string // used as key
	OOMPoints int
	Rank      int
	Result    string // stableford, gross, net, or bogey result as displayed on web
}

// Competition describes the competition and all the players results
type Competition struct {
	// The first set of fields can be parsed from the 'list of comps' webpage
	Key        string
	Name       string
	Date       string
	URL        string
	// The remaining fields can be populated from the web page for this competition
	NumPlayers int
	Results    map[string]PlayerResult // key by player name
}

// FIRST SECTION OF FILE DEALS WITH BUILDING LIST OF COMPETITIONS

// The call tree given no files cached and a list of competitions specified
// in the call to FetchCompDescriptions(2016, "oom.conf") is depicted below, noting
// that a subsequent run would use the files cached by the first run:
//
//  FetchCompDescriptions(2016, "oom.conf")
//    parseKeysFromFile("oom.conf")
//      loop per line: parseNextCompKey()
//    fetch competition list page from cgc and cache in all_comps.dat
//    parseComps(content of all_comps.dat)
//    update description field (possibly excepting URL) with data from web page
//    return a []Competition where just the descriptive fields are populated
//

// FetchCompDescriptions returns a []Competition with the descriptive set of
// fields filled in, for the list of competition keys provided in the
// file passed as a parameter fname.  The fields are populated using
// data from the website - except if a valid URL is provided
// in the parameter file, in which case it is used.  This allows manual
// tweaking - for example to tell the website to return the net rather
// than the default gross scores for the club chanmpionships.
// FIRST the list of keys is read from the paramter file,
// THEN the details are augmented/overwritten by data from the website
// (excepting the URL as desribed above)
// GOTCHA regards caching: the saved web page with 'all comps' may be
// out of date meaning the latest competition is not listed.  In this case
// we need to attempt to re-read from the web and try again.  If the key is
// still not found this implies an error in oom.conf (e.g. a non-existant
// competition has been asked for)

// return first key in slice that is not in the map
func firstMissingKey(c []Competition, m map[string]Competition) (bool, string) {
	var missing bool
	var missingKey string
	for _, cComp := range c {
		if _, ok := m[cComp.Key]; ok == false {
			missing = true
			missingKey = cComp.Key
			break
		}
		if missing {break}
	}
	return missing, missingKey
}

func FetchCompDescriptions(year int, fname string) []Competition {
  oomCompetitions := parseKeysFromFile(fname) // may also set URL, is a slice
	d, fromCache := fetchAllCompsPage(year, true) // noting cache may be stale
  allCompetitions := parseWebComps(string(d)) // may be stale, is a map
	// check all the comp keys from the file are found in the web page
	missing, missingKey := firstMissingKey(oomCompetitions, allCompetitions)
	if missing && !fromCache {
		log.Fatal("Competition id %s not found on web site list of comps",
			missingKey)
	} else {
		if missing && fromCache {
			// read from web and try again
			d, fromCache = fetchAllCompsPage(year, false)
			allCompetitions = parseWebComps(string(d))
			missing, missingKey := firstMissingKey(oomCompetitions, allCompetitions)
			if missing && !fromCache {
				log.Fatal("Competition id ", missingKey,
					" not found on web site list of comps")
			}
		}
	}
  // update the oomCompDescs to include the name and date from the web
  // if the oomComDescs already has a valid url, keep it, otherwise
  // take the url from allCompsDescs post-pended with &sort=0 for net score ranking
  for n, oomCompetition := range oomCompetitions {
    for _, competition := range allCompetitions {
      if oomCompetition.Key == competition.Key {
        oomCompetitions[n].Name = competition.Name
        oomCompetitions[n].Date = competition.Date
        if oomCompetitions[n].URL == "" {
          oomCompetitions[n].URL = competition.URL + "&sort=1" // this sort seems to get net results...
        } // otherwise use the url as read from the file
        // TODO break out
      }
    }
  }
  return oomCompetitions
}

func fetchAllCompsPage(year int, useCached bool) (d []byte, fromCache bool) {
	fname := fmt.Sprintf("all_comps_%d.dat", year)
	if useCached {
		d1, err := ioutil.ReadFile(fname)
		if err == nil {
			d = d1 // this is where := is a bit crappy
			fromCache = true
			return
		}
	}
  url := fmt.Sprintf("http://www.colchestergolfclub.com/competition.php?showall=1&time=&show=&year=%d", year)
  d = MustFetch(url)
  ioutil.WriteFile(fname, d, 0644)
	return
}

// fetchAllCompDesc returns a []Competition with the first descriptive set of
//  fields filled in.  All competitions from the given year are populated
// TODO use cached all_comps.dat
func FetchAllCompDesc(year int) []Competition {
  log.Println("building competition descriptions...")
	d, _ := fetchAllCompsPage(year, true)
	cMap := parseWebComps(string(d))
	var cSlice []Competition
	for _, v := range cMap {
		cSlice = append(cSlice, v)
	}
	return cSlice // can't win as want map some places and slices in others
}


// parseKeysFromFile reads the file and populates the Key field,
// returning a []Competition.
// If the URL read from file appears valid (a whole URL, not just the
// key fragment), it is populated in URL
func parseKeysFromFile(fname string) []Competition {
  file, err := os.Open(fname)
  if err != nil {
    log.Fatal(err)
  }
  defer file.Close()

  var ret []Competition
  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    compid, _ := parseNextCompKey(scanner.Text(), 0)
    if compid != "" {
      desc := Competition{Key: compid}
      // now test if the ?compid= is part of a valid url, and if so
      // put the url in the desc
      s := strings.TrimSpace(strings.Split(scanner.Text(), ",")[1])
      if u, err := url.Parse(s); err == nil {
        if u.Scheme == "http" {
          desc.URL = s
        }
      }
      ret = append(ret, desc)
    }
  }
	if err := scanner.Err(); err != nil {
    log.Fatal(err)
  }
  return ret
}

// used with parseNextCompid
// TODO nest this function
func endInt(s string, start int) int {
	for start < len(s) {
		switch s[start:start+1] {
			case "0","1","2","3","4","5","6","7","8","9":
				start++
			default:
				return start
		}
	}
	return start
}
func parseNextCompKey(s string, from int) (string, int) {
	tok := "?compid="
	start := strings.Index(s[from:], tok)
	if start == -1 {
		return "", -1
	}
	start += (from + len(tok))
	end := endInt(s, start)
	return s[start:end], end
}

// build a map keyed on comppId
func parseWebComps(compstr string) map[string]Competition {
  var ret = make(map[string]Competition)
  for start := tokenStart(compstr, "?compid="); start != -1;
        start = tokenStart(compstr, "?compid=") {
    end := tokenEnd(compstr, start, "\"")
    compid := compstr[start:end]
    compstr = compstr[end:]
    start = tokenStart(compstr, "\">")
    end = tokenEnd(compstr, start, "</a>")
    compname := compstr[start:end]
    compstr = compstr[end:]

    start = tokenStart(compstr, "<td>")
    end = tokenEnd(compstr, start, "</td>")
    compdate := compstr[start:end]
    compstr = compstr[end:]

    ret[compid] = Competition{Key: compid, Name: compname, Date: compdate,
        URL: fmt.Sprintf(
				 "http://www.colchestergolfclub.com/competition.php?compid=%s", compid)}
  }
  return ret
}

func tokenStart(s string, tok string) int {
  i := strings.Index(s, tok)
  if i == -1 { return -1 }
  return i + len(tok)
}
func tokenEnd(s string, start int, terminator string) int {
  i := strings.Index(s[start:], terminator)
  if i == -1 { return -1 }
  return start + i
}


// SECOND SECTION OF FILE DEALS WITH POPULATING COMPETITION RESULTS

// Load populates the comptition identifed by the comp.Key.
// A valid comp.URL is required unless the results are already cached
// The competition is read from the cached file 'key.txt' if present.
// Otherwise the web page is fetched, parsed, and the cached file created.
// The optional urlString is used if supplied, otherwise a default
// url is constructed based on the key
func Load(comp *Competition) {
	if comp.Key == "" {
		err := errors.New("competition.Load: Invalid null competetiton key supplied")
		log.Fatal(err)
	}
	if readCached(comp) { return }
	populateResultsFromWeb(comp)
	saveComp(comp)
}


// readCached returns false if there is no cached file, otherwise the
// Competition is returned along with true
func readCached(comp *Competition) bool {
	fname := comp.Key + ".txt"
	f, err := os.Open(fname)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	_ = scanner.Scan()
	for scanner.Text()[0:1] == "#" {
		_ = scanner.Scan()
	}
	comp.Key = strings.TrimSpace(strings.Split(scanner.Text(), ",")[1])
	_ = scanner.Scan()
	comp.Name = strings.TrimSpace(strings.Split(scanner.Text(), ",")[1])
	_ = scanner.Scan()
	comp.Date = strings.TrimSpace(strings.Split(scanner.Text(), ",")[1])
	_ = scanner.Scan()
	comp.URL = strings.TrimSpace(strings.Split(scanner.Text(), ",")[1])
	_ = scanner.Scan()
	comp.NumPlayers, _ = strconv.Atoi(strings.TrimSpace(strings.Split(scanner.Text(), ",")[1]))
	_ = scanner.Scan()
	// ignore the header row
	comp.Results = make(map[string]PlayerResult)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), ",")
		var playerResult PlayerResult
		playerResult.Name = strings.TrimSpace(s[3])
		playerResult.Result = strings.TrimSpace(s[2])
		playerResult.Rank, _ = strconv.Atoi(strings.TrimSpace(strings.Split(scanner.Text(), ",")[1]))
		playerResult.OOMPoints, _ = strconv.Atoi(strings.TrimSpace(strings.Split(scanner.Text(), ",")[0]))
		comp.Results[playerResult.Name] = playerResult
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return true
}

// saveComp creates a cache file for the competition that can be read back in
func saveComp(comp *Competition) {
  fname:= fmt.Sprintf("%s.txt", comp.Key)
  f, err := os.Create(fname)
  if err != nil {log.Fatal(err)}
  defer f.Close()
  eol := "\r\n"
  fmt.Fprint(f, "key, ", comp.Key, eol)
  fmt.Fprint(f, "name, ", comp.Name, eol)
  fmt.Fprint(f, "date, ", comp.Date, eol)
  fmt.Fprint(f, "url, ", comp.URL, eol)
  fmt.Fprint(f, "number of players, ", comp.NumPlayers, eol)
  fmt.Fprint(f, "oom_points, rank_in_comp, igresult, name - row per player", eol)
  for _,p := range comp.Results {
    s := fmt.Sprintf("%10v, %12v, %8v, %v, %s", p.OOMPoints, p.Rank, p.Result, p.Name, eol)
    fmt.Fprint(f, s)
  }
}

// populateResultsFromWeb gets the page pointed by Competition.URL, and parses the
// results in to the passed Competition
func populateResultsFromWeb(comp *Competition) {
	data := MustFetch(comp.URL)
  // Have seen two formats for web page
  // 1. use of ?playerid= used for most competitions
  // 2. use of class="namecol" for the club championships with two rounds
  scanner := bufio.NewScanner(bytes.NewReader(data))
  splitfn := compSplitFunc // splitter for normal format competitions
  detail := playerDetail // extract player result for normal format
  if -1 == strings.Index(string(data), "?playerid=") {
    splitfn = champSplitFunc // splitter for club champtionship formatted comps
    detail = champDetail // extract player result for championship format
  }
  scanner.Split(splitfn)
  numPlayers := 0

  var res []PlayerResult
  first := true
	for scanner.Scan() {
		if first {
      first = false // scan and discard page up to start of first player result
    } else {
      numPlayers++
      var player PlayerResult
      name, result := detail(scanner.Text())
      player.Name = name
      player.Result = result
      player.Rank = numPlayers
      res = append(res, player)
    }
	}
  comp.NumPlayers = numPlayers
  comp.Results = make(map[string] PlayerResult)
  for n, p := range res {
    p.OOMPoints = numPlayers - n
    if _, err := strconv.Atoi(p.Result); err != nil {  // DQ, NR...
      p.OOMPoints = 0
    }
    comp.Results[p.Name] = p
  }
}

func compSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
    // Return nothing if at end of file and no data passed
    if atEOF && len(data) == 0 {
        return 0, nil, nil
    }

    if i := strings.Index(string(data), "?playerid="); i >= 0 {
        return i + 1, data[0:i], nil
    }

    // If at end of file with data return the data
    if atEOF {
        return len(data), data, nil
    }
    return
}

func champSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
    // Return nothing if at end of file and no data passed
    if atEOF && len(data) == 0 {
        return 0, nil, nil
    }

    if i := strings.Index(string(data), "class=\"namecol\">"); i >= 0 {
        return i + 1, data[0:i], nil
    }

    // If at end of file with data return the data
    if atEOF {
        return len(data), data, nil
    }
    return
}

//?playerid=76041">Jo Mager</a>(16)</td>
//<td><a href="viewround.php?roundid=16413" title="Countback results: Back 9 - 12, Back 6 - 8, Back 3 - 4, Back 1 - 2">24</a></td>
//<td></td>
//</tr>
func playerDetail(s string) (name string, score string) {
    start := strings.Index(s, ">")
    end := strings.Index(s, "</a>")
    name = s[start + 1:end]

    s = s[end:]
    end = strings.Index(s, "</a></td>")
    s = s[:end]
    start = strings.LastIndex(s, ">")
    score = s[start + 1:]
    return
}

func champDetail(s string) (name string, score string) {
    start := strings.Index(s, ">")
    end := strings.Index(s, "<") // fragile!!
    end2 := strings.Index(s, "(")
    if end2 != -1 && end2 < end {
      end = end2
    }
    // this may include handicap
    name = strings.TrimSpace(s[start + 1:end])
    s = s[end:]
    end = strings.Index(s, "</td></tr>")
    if end == -1 {
      fmt.Println(s)
      os.Exit(1)
    }
    s = s[:end]
    if "</span>" == s[len(s)-7:] {
      s = s[:len(s) - 7]
    }
    start = strings.LastIndex(s, ">")
    score = s[start + 1:]
    if "&nbsp;" == score {
      score = "NS"
    }
    return
}
