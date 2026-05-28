import { runAskCommand } from "./commands/ask.js";

const [, , command, ...args] = process.argv;

if (command === "ask") {
  const question = args.join(" ").trim();
  if (!question) {
    usage(1);
  }
  await runAskCommand({ question });
} else {
  usage(command ? 1 : 0);
}

function usage(exitCode: number): never {
  process.stderr.write("Usage: pnpm cli ask <question>\n");
  process.exit(exitCode);
}
