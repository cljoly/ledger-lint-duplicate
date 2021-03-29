#!/usr/bin/env python3

import sys
from xml.dom import minidom
from collections import namedtuple


Tx = namedtuple('Tx', ['date', 'postings'])
Posting = namedtuple('Posting', ['account_name', 'quantity'])


def createTx(xmlTx: minidom.Element):
    date = xmlTx.getElementsByTagName('date')[0].data
    posting = (0,)
    return Tx(date, posting)


if __name__ == '__main__':
    file = minidom.parse(sys.argv[1])
    txs = file.getElementsByTagName('transaction')
    tx = txs[0]
    print(createTx(tx))
