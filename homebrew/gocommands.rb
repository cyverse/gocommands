class Gocommands < Formula
  desc "Portable command-line toolkit for iRODS data management service"
  homepage "https://github.com/cyverse/gocommands"
  url "https://github.com/cyverse/gocommands/archive/refs/tags/v0.8.1.tar.gz"
  sha256 "9b9bbc34484413b16af45435258a4a0d7bf33e678718c12bdfc981577da058a1"
  license "BSD-3-Clause"

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  depends_on "go" => :build

  def install
    gocmd_pkg = "github.com/cyverse/gocommands"
    gocmd_version = version.to_s
    ldflags = "-X #{gocmd_pkg}/commons.clientVersion=v#{gocmd_version}"

    system "go", "build", "-ldflags", ldflags, "-o", "gocmd", "cmd/gocmd.go"

    bin.install "gocmd"
  end

  test do
    assert_match "clientVersion",
      shell_output("#{bin}/gocmd --version")
  end
end
