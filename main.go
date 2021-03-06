/*
	ledger lint duplicate finds duplicates transactions in your ledger file.
	Copyright © 2021 Clément Joly

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"time"

	"zgo.at/zli"
)

type Ledger struct {
	XMLName     xml.Name `xml:"ledger"`
	Text        string   `xml:",chardata"`
	Version     string   `xml:"version,attr"`
	Commodities struct {
		Text      string `xml:",chardata"`
		Commodity struct {
			Text   string `xml:",chardata"`
			Flags  string `xml:"flags,attr"`
			Symbol string `xml:"symbol"`
		} `xml:"commodity"`
	} `xml:"commodities"`
	Accounts struct {
		Text    string `xml:",chardata"`
		Account struct {
			Text         string `xml:",chardata"`
			ID           string `xml:"id,attr"`
			Name         string `xml:"name"`
			Fullname     string `xml:"fullname"`
			AccountTotal struct {
				Text   string `xml:",chardata"`
				Amount struct {
					Text     string `xml:",chardata"`
					Quantity string `xml:"quantity"`
				} `xml:"amount"`
			} `xml:"account-total"`
			Account []struct {
				Text         string `xml:",chardata"`
				ID           string `xml:"id,attr"`
				Name         string `xml:"name"`
				Fullname     string `xml:"fullname"`
				AccountTotal struct {
					Text   string `xml:",chardata"`
					Amount struct {
						Text     string `xml:",chardata"`
						Quantity string `xml:"quantity"`
					} `xml:"amount"`
				} `xml:"account-total"`
				Account struct {
					Text          string `xml:",chardata"`
					ID            string `xml:"id,attr"`
					Name          string `xml:"name"`
					Fullname      string `xml:"fullname"`
					AccountAmount struct {
						Text   string `xml:",chardata"`
						Amount struct {
							Text     string `xml:",chardata"`
							Quantity string `xml:"quantity"`
						} `xml:"amount"`
					} `xml:"account-amount"`
					AccountTotal struct {
						Text   string `xml:",chardata"`
						Amount struct {
							Text     string `xml:",chardata"`
							Quantity string `xml:"quantity"`
						} `xml:"amount"`
					} `xml:"account-total"`
				} `xml:"account"`
			} `xml:"account"`
		} `xml:"account"`
	} `xml:"accounts"`
	Transactions struct {
		Text        string `xml:",chardata"`
		Transaction []struct {
			Text     string `xml:",chardata"`
			State    string `xml:"state,attr"`
			Date     string `xml:"date"`
			Payee    string `xml:"payee"`
			Note     string `xml:"note"`
			Metadata struct {
				Text  string `xml:",chardata"`
				Value []struct {
					Text   string `xml:",chardata"`
					Key    string `xml:"key,attr"`
					String string `xml:"string"`
				} `xml:"value"`
				Tags []string `xml:"tag"`
			} `xml:"metadata"`
			Postings struct {
				Text    string `xml:",chardata"`
				Posting []struct {
					Text    string `xml:",chardata"`
					State   string `xml:"state,attr"`
					Virtual string `xml:"virtual,attr"`
					Account struct {
						Text string `xml:",chardata"`
						Ref  string `xml:"ref,attr"`
						Name string `xml:"name"`
					} `xml:"account"`
					PostAmount struct {
						Text   string `xml:",chardata"`
						Amount struct {
							Text     string  `xml:",chardata"`
							Quantity float64 `xml:"quantity"`
						} `xml:"amount"`
					} `xml:"post-amount"`
					BalanceAssignment struct {
						Text     string  `xml:",chardata"`
						Quantity float64 `xml:"quantity"`
					} `xml:"balance-assignment"`
					Total struct {
						Text   string `xml:",chardata"`
						Amount struct {
							Text     string  `xml:",chardata"`
							Quantity float64 `xml:"quantity"`
						} `xml:"amount"`
					} `xml:"total"`
				} `xml:"posting"`
			} `xml:"postings"`
		} `xml:"transaction"`
	} `xml:"transactions"`
}

func (l *Ledger) toTxs() map[float64][]Tx {
	txs := make(map[float64][]Tx)
	for p, txXml := range l.Transactions.Transaction {
		date, err := time.Parse("2006/01/02", txXml.Date)
		if err != nil {
			log.Fatal(err)
		}

		for _, posting := range txXml.Postings.Posting {
			amount := posting.PostAmount.Amount.Quantity

			tags := make([]string, len(txXml.Metadata.Tags), len(txXml.Metadata.Tags))
			copy(tags, txXml.Metadata.Tags)

			tx := Tx{
				Date:     date,
				Position: p,
				Payee:    txXml.Payee,
				Account:  posting.Account.Name,
				Amount:   amount,
				Tags:     tags,
			}

			subTxs, exists := txs[amount]
			if exists {
				txs[amount] = append(subTxs, tx)
			} else {
				txs[amount] = []Tx{tx}
			}
		}
	}
	return txs
}

type Tx struct {
	Date time.Time
	// Position in the imported xml file
	Position int
	Payee    string
	Account  string
	Amount   float64
	Tags     []string
}

// Find returns true on the first encountered occurence of val in slice
func find(val string, slice []string) bool {
	for _, str := range slice {
		if str == val {
			return true
		}
	}
	return false
}

func printDuplicate(ignoredTag string, txs ...*Tx) {
	if len(txs) <= 0 {
		return
	}

	fmt.Print(zli.BrightBlack|zli.White.Bg(), "; Potential duplicates:", zli.Reset, "\n")
	for _, tx := range txs {
		var tagIndicator string
		if find(ignoredTag, tx.Tags) {
			tagIndicator = fmt.Sprint(zli.Blue, "[IGNORED]", zli.Reset)
		}

		fmt.Printf("(%v)\t%v %v\t\t\t%v\n\t\t%v\t\t\t%v\n",
			tx.Position, tx.Date.Format("2006-01-02"), tx.Payee, tagIndicator,
			tx.Account, tx.Amount)
	}
}

// maxDuration is in hours
func findDuplicates(maxDuration float64, ignoredTag string, txs map[float64][]Tx) (allDuplicates [][]*Tx) {
	// Add duplicates, unles all transactions are marked with the ignore tag
	keep := func(duplicates []*Tx) {
		// If all duplicates have the ignore tag, drop them
		for _, tx := range duplicates {
			if !find(ignoredTag, tx.Tags) {
				allDuplicates = append(allDuplicates, duplicates)
				return
			}
		}
	}

	for _, txs := range txs {
		if len(txs) <= 1 {
			continue
		}

		sort.SliceStable(txs, func(i, j int) bool {
			return txs[i].Date.Before(txs[j].Date)
		})

		var duplicates []*Tx
		lastInserted := -1
		for i := 1; i < len(txs); i++ {
			endDate := txs[i].Date
			d := txs[i].Date.Sub(txs[i-1].Date)
			if d.Hours() <= maxDuration {
				if d.Hours() < 0 {
					log.Fatal("negative duration 1, this is a bug, please report it!")
				}
				if lastInserted >= 0 && endDate.Sub(duplicates[lastInserted].Date).Hours() <= maxDuration {
					if endDate.Sub(duplicates[lastInserted].Date).Hours() < 0 {
						log.Fatal("negative duration 2, this is a bug, please report it!")
					}
					duplicates = append(duplicates, &txs[i])
					lastInserted++
				} else {
					keep(duplicates)
					duplicates = []*Tx{&txs[i-1], &txs[i]}
					lastInserted = 1
				}
			}
		}

		keep(duplicates)
	}
	return allDuplicates
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
var days = flag.Float64("days", 10, "time in days to take before and after for two transactions to be considered duplicate")
var ignoredTag = flag.String("ignore-tag", "notDup", "ignore these tags when all duplicates transactions have it")

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// TODO Support multiple flag names
	fileNames := flag.Args()
	fileName := fileNames[0]
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	var ledger Ledger
	xml.Unmarshal(b, &ledger)

	txs := ledger.toTxs()
	duplicates := findDuplicates(24.**days, *ignoredTag, txs)
	for _, d := range duplicates {
		printDuplicate(*ignoredTag, d...)
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
