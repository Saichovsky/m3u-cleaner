# m3u-cleaner
Python based m3u playlist cleaner.

This script checks uses `aiohttp` for asyncronous checks to quicken the cleanup. It also uses the HEAD method to check server availability as well as TCP checks on each channel in the list.

The script uses the default location of `/data` to scan for `*.m3u` files which are merged into one file `index.m3u` file with valid channel links only.
