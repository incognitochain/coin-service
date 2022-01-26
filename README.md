# incognito-coin-worker


## Running your own coinservice

The following steps will guide you to run your own coinservice:

### Prerequisite:
```
 - Linux/macOS machine
 - Docker
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
```
### Step 4: Edit nginx config at `deploy/nginx/nginx.conf`
```
This file contain routing rules of coinservice, you can set limit or denied access to the api(s) here.
```
### Step 5: Run `rundocker.sh`
```
./rundocker.sh NGINX_PORT_NUMBER_YOUR_WANTED

ex: ./rundocker.sh 9293
```

### Stopping coinservice
```
cd deploy
docker-compose down
```