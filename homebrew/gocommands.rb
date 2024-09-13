class Gocommands < Formula
  desc "Portable command-line toolkit for iRODS data management service"
  homepage "https://github.com/cyverse/gocommands"
  url "https://github.com/cyverse/gocommands/archive/refs/tags/v0.10.0.tar.gz"
  sha256 "abf52ea4c0091f8faabb220a1534e825a7981b92acc44a01cc7e29e23778759a"
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
