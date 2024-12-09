import { IMClient } from "./sdk";

console.log('Hello from js-sdk/index.ts');

const main = async () => {
    let cli = new IMClient("ws://localhost:8080","ccc");
    let {status} = await cli.login();
    console.log("client login return --- ", status);
}


main();