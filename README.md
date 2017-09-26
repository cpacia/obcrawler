# obcrawler
OpenBazaar network crawler

To run this you must have an openbazaar-go node running on localhost.

### Install
```
go get github.com/cpacia/obcrawler
```

### Usage
```
obcrawler start
```

`curl http://localhpst:8080/peers`
```
[
    "QmUD1jyaRmpJchLXXWUf9ZeuG4ZaFjMm8uY1XDakKtBExh",
    "QmXBafvHmZeQ1D3jrJAs5h7XRWyt9MV8oc24C3vHPAGRK3",
    "QmWegmpWfCQp6dwh4riAHhvuYETM2kTroTNr4ELmo4RRSv",
    "QmTDmUk7ceNhyfMqdyFuzCukKDM4KgpZZFNaCG1GJ2Uby3",
    "QmTvtYb818uYfCAKHVt6CvqRRmceNUDEwoYho4Z8RZYqsT",
    "QmZtDNYXSJzKMNCEjiRxBcC2eZ19u4fuYJRsA9MuGBggcz",
    "QmRsWkK6pG87g8NrADdWoRqFJ3jubF2nJvsUksUkWqjEoc",
    "QmbcREmuCwXR8WKW3tBPukaXVGLA3Zc2JhavUwFD6qkBHB",
    "QmNNU49ebcf7hvszwz267qXmk2sz6Bbog7tdC56k2DZjNQ"
]
```

`curl http://localhpst:8080/peers?only=tor`

`curl http://localhpst:8080/peers?only=dualstack`

`curl http://localhpst:8080/peers?only=clearnet`

`curl http://localhpst:8080/count`
```
259
```

`curl http://localhpst:8080/count?lastActive=1d`

`curl http://localhpst:8080/count?lastActive=1d&only=tor`

`curl http://localhpst:8080/useragents`
```
{
    "/openbazaar-go:0.6.0/": 1,
    "/openbazaar-go:0.7.0/": 3,
    "/openbazaar-go:0.7.0/'ob1:v0.7.0d'": 5,
    "/openbazaar-go:0.7.2/": 2,
    "/openbazaar-go:0.7.2/'ob1:v0.7.0'": 1,
    "/openbazaar-go:0.7.3/": 9,
    "/openbazaar-go:0.7.3/'ob1:v0.7.0d'": 1,
    "/openbazaar-go:0.7.3/'ob1:v0.7.0f'": 1,
    "/openbazaar-go:0.7.3/ob1": 57,
    "/openbazaar-go:0.8.0/": 1,
    "/openbazaar-go:0.9.0/": 4,
    "/openbazaar-go:0.9.1/": 169,
    "/openbazaar-go:0.9.2/": 12
}
```


