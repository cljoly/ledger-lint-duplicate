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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"
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
				Value struct {
					Text   string `xml:",chardata"`
					Key    string `xml:"key,attr"`
					String string `xml:"string"`
				} `xml:"value"`
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
	for p, tx := range l.Transactions.Transaction {
		date, err := time.Parse("2006/01/02", tx.Date)
		if err != nil {
			log.Fatal(err)
		}

		for _, posting := range tx.Postings.Posting {
			amount := posting.PostAmount.Amount.Quantity

			tx := Tx{
				Date:     date,
				Position: p,
				Payee:    tx.Payee,
				Account:  posting.Account.Name,
				Amount:   amount,
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
}

func printDuplicate(printed *map[int]bool, txs ...Tx) {
	if len(txs) <= 0 {
		return
	}

	fmt.Println("Potential new duplicates:")
	for _, tx := range txs {
		fmt.Printf("(%v)\t%v %v\n\t\t%v\t\t\t%v\n",
			tx.Position, tx.Date.Format("2006-01-02"), tx.Payee,
			tx.Account, tx.Amount)
	}
	fmt.Println()
}

func findDuplicates(txs map[float64][]Tx) (allDuplicates [][]Tx) {
	for _, txs := range txs {
		if len(txs) <= 1 {
			continue
		}

		sort.SliceStable(txs, func(i, j int) bool {
			return txs[i].Date.Before(txs[j].Date) || txs[i].Account < txs[j].Account
		})

		var duplicates []Tx
		for i := 1; i < len(txs); i++ {
			endDate := txs[i].Date
			d := txs[i].Date.Sub(txs[i-1].Date)
			if d.Hours() <= TenDaysInHours {
				if len(duplicates) >= 1 && endDate.Sub(duplicates[len(duplicates)-1].Date).Hours() <= TenDaysInHours {
					duplicates = append(duplicates, txs[i])
				} else {
					allDuplicates = append(allDuplicates, duplicates)
					duplicates = []Tx{txs[i-1], txs[i]}
				}
			}
		}

		allDuplicates = append(allDuplicates, duplicates)
	}
	return allDuplicates
}

const TenDaysInHours = 240.0

func main() {
	fileName := os.Args[1]
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	var ledger Ledger
	xml.Unmarshal(b, &ledger)

	txs := ledger.toTxs()
	duplicates := findDuplicates(txs)
	printed := make(map[int]bool)
	for _, d := range duplicates {
		printDuplicate(&printed, d...)
	}
}
