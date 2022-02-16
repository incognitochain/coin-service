# incognito-coin-worker


## Running your own coinservice

The following steps will guide you to run your own coinservice:

### Prerequisite:
```
 - Linux/macOS machine
 - Docker & docker compose
 - Golang 1.17
```

### Step 1: Clone this repo
```
git clone github.com/incognitochain/coin-service
```
### Step 2: Switch to release branch
```
git checkout docs
```
### Step 3: Edit env config at `deploy/service.env`
```
This file contain information about mongo access, which network coinservice will sync from, internal service port and some external services.

Note:
CHAINNETWORK: corresponding to the name of each config folder in deploy/csv/config
EXDECIMALS: external service to get the correct decimal of a bridge token
ANALYTICS: external service to get price history(chart) and price change. 
```
### Step 4: Edit nginx config at `deploy/nginx/nginx.conf`
```
This file contain routing rules of coinservice, you can set limit or denied access to the api(s) here.
```
### Step 5: Run `runservice.sh`
```
./runservice.sh NGINX_PORT_NUMBER_YOUR_WANTED

ex: ./runservice.sh 9293
```

### Stopping coinservice
```
cd deploy
docker-compose down
```