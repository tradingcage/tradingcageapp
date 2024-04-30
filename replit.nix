{ pkgs }: {
  deps = [
    pkgs.gotools
    pkgs.wget
    pkgs.ack
    pkgs.postgresql
    pkgs.sqlite.bin
    pkgs.jq
  ];
}