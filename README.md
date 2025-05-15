# blog-aggregator

## Software Requirements to Run
1. Go installed
2. Postgres installed

## How to Install
1. From downloaded directory, run the following to build the application:
    `$ go build`
2. From downloaded directory, run the following to find the install path:
    `$ go list -f '{{.Target}}'`
3. Run the following to install the application:
    `$ go install`
4. Navigate to the install path from step 2 and run the following to verify the application is installed (should return "missing arguments"):
    `$ blog-aggregator`

## How to Use
### List of Commands
1. register [username]
2. login [username]
3. addfeed [name] [url]
...