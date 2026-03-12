const go = new Go();
go.argv = ["gddoom", "-wad=./DOOM1.WAD", "-opl3-backend=purego"];

fetch("./gddoom.wasm")
  .then((response) => response.arrayBuffer())
  .then((bytes) => WebAssembly.instantiate(bytes, go.importObject))
  .then((result) => {
    go.run(result.instance);
  });
