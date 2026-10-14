"use strict";

var fs = require("fs");

// npm 10+ runs install scripts in the background and hides their stdout.
// Write the next step to the terminal when there is one.
var message = "oops init\n";
var paths = process.platform === "win32" ? ["\\\\.\\CON", "/dev/tty"] : ["/dev/tty"];
for (var i = 0; i < paths.length; i++) {
	try {
		var fd = fs.openSync(paths[i], "w");
		fs.writeSync(fd, message);
		fs.closeSync(fd);
		process.exit(0);
	} catch (err) {
		// No controlling terminal. Fall through to stdout.
	}
}
process.stdout.write(message);
