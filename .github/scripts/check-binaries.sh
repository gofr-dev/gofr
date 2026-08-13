#!/usr/bin/env bash
# Fail if any tracked file is a compiled executable.
#
# Two independent signals, because either alone has a hole:
#   - mode 100755      catches build output whatever its contents
#   - executable magic catches a binary committed with mode 100644
#
# Matches executable formats only (ELF / Mach-O / PE), never "is binary" —
# the repo legitimately tracks images and minified JS.
#
# Every blob is streamed through a single `git cat-file --batch`, so this is
# one subprocess regardless of repository size.

set -uo pipefail

ref="${1:-HEAD}"
fail=0

# ---- signal 1: the executable bit -------------------------------------------
# No tracked file in this repo legitimately carries mode 100755. If that ever
# changes, allowlist the specific path rather than dropping the check.
while IFS= read -r path; do
  [ -n "$path" ] || continue
  printf '  executable bit set: %s\n' "$path"
  fail=1
done < <(git ls-tree -r "$ref" | awk -F'\t' '$1 ~ /^100755/ { print $2 }')

# ---- signal 2: executable file magic ---------------------------------------
magic_hits=$(
  git ls-tree -r "$ref" |
  perl -MIPC::Open2 -e '
    my (%path, @oids);
    while (<STDIN>) {
      chomp;
      my ($meta, $p) = split /\t/, $_, 2;
      my (undef, $type, $oid) = split /\s+/, $meta;
      next unless $type eq "blob";
      $path{$oid} = $p;
      push @oids, $oid;
    }
    my ($out, $in);
    open2($out, $in, "git", "cat-file", "--batch");
    binmode $out;
    for my $oid (@oids) { print $in "$oid\n" }
    close $in;

    my %fmt = (
      "\x7fELF"          => "ELF",
      "\xfe\xed\xfa\xce" => "Mach-O", "\xce\xfa\xed\xfe" => "Mach-O",
      "\xfe\xed\xfa\xcf" => "Mach-O", "\xcf\xfa\xed\xfe" => "Mach-O",
      "\xca\xfe\xba\xbe" => "Mach-O universal",
      "\xbe\xba\xfe\xca" => "Mach-O universal",
    );

    while (my $hdr = <$out>) {
      my ($oid, $type, $size) = split /\s+/, $hdr;
      last unless defined $size;
      my $buf = "";
      read($out, $buf, $size) if $size > 0;
      read($out, my $lf, 1);

      my $head = substr($buf, 0, 4);
      my $f = $fmt{$head};
      $f = "PE" if !defined($f) && substr($buf, 0, 2) eq "MZ";
      next unless defined $f;
      print "  $f executable: $path{$oid}\n";
    }
  '
)

if [ -n "$magic_hits" ]; then
  printf '%s\n' "$magic_hits"
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  cat <<'MSG'

Compiled executables must not be committed.
`go build` inside a package main directory writes an extensionless binary
named after that directory. Build with `-o` into a temp path, or delete the
artifact before committing.
MSG
  exit 1
fi

echo "no committed executables"
