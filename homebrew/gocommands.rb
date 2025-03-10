class Gocommands < Formula
  desc "Portable command-line toolkit for iRODS data management service"
  homepage "https://github.com/cyverse/gocommands"
  url "https://github.com/cyverse/gocommands/archive/refs/tags/v0.10.18.tar.gz"
  sha256 "d5558cd419c8d46bdc958064cb97f963d1ea793866414c025906ec15033512ed"
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
