class Gocommands < Formula
  desc "Portable command-line toolkit for iRODS data management service"
  homepage "https://github.com/cyverse/gocommands"
  url "https://github.com/cyverse/gocommands/archive/refs/tags/v0.6.5.tar.gz"
  sha256 "a9847bd246f7ab5359e54cca4910f7f11b9a2b5453d572babc3a063d0e8f9a4d"
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
