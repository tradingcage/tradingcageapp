# Trading Cage

Trading Cage is a futures trading simulator that I worked on for a few months towards the end of 2023 and beginning of 2024. After initially launching it as a paid subscription product, I decided to wind it down and pursue other projects. Now that it's defunct, I am opening up the code in case anyone out there finds it useful. I provide all code under the MIT license so feel free to do whatever you want with it.

## Product Demonstration

Watch a product demonstration on YouTube to see how the application works.

[![Trading Cage Product Demonstration Video](http://img.youtube.com/vi/30T-FvJ-cqw/0.jpg)](http://www.youtube.com/watch?v=30T-FvJ-cqw "Trading Cage Product Demonstration")

## Code

The code may be difficult to read -- it's not in a particularly usable state, but there may be pieces a skilled programmer can extract and get value from. What follows is a loose description of the main directories.

### Root directory

The `.replit` and `replit.nix` files are intended to be used to develop the code on Replit, the cloud IDE.

`build.sh` and `run.sh` are used to build and run the code.

`go.mod` and `go.sum` are standard Go modules files.

`main.go` is the main entrypoint of the server.

### Static

The `static` directory includes static assets such as images, icons, and JavaScript dependencies, including [FirChart](https://github.com/tradingcage/firchart), the open-source charting library.

### Templates

Trading Cage uses Go's HTML templates for each page. Some of these pages also include Svelte entrypoints which are bundled from the `frontend` directory.

### pkg

This is the standard directory for Go library code. In this directory, find all serverside implementations referenced in main.go, including billing, database queries, trade simulation, user registration, and so on.

### Frontend

This is where the Svelte frontend code lives. It's organized in a fairly standard way and uses the API endpoints to interact with the server.
