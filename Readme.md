## Asset Portfolio analyzer using Binance API + CCData.io API

I'm an avid crypto(asset) user, investor & engineer. I've tons of assets and it is hard, really hard to keep track and make rapid decisions in this volatile market without lots of context switching between tabs, even my head hurts [can't imagine how those browser tabs feel too ;)]

I built this to have overview and analysis of my assets to make informed decision.

### Built with Golang + REST API + Templates

Things you need:

-   Go runtime
-   Internet connection (just this one time I promise!)
-   Binance api & secret keys (for personal account REST endpoints)
-   CCData.io api key (for rate-limits)

#### How to Run

```bash
cd to/your/clone/path
nano .env [update with your api keys]
make run or go run cmd/*.go
```

#### Terminology:

1. _Daily PNL_: Calculated using the 24-hour price change.
2. _Unrealized PNL_: Difference between current price and average buy price for the holding.
3. _Realized PNL_: Sum of all profits/losses on completed trades.
4. _Portfolio Allocation_: Percentage of total portfolio value allocated to the asset.

## License

All non-crypto rights reserved!
