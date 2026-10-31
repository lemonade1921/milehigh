# Homebrew formula. This repository doubles as a tap:
#   brew tap lemonade1921/milehigh https://github.com/lemonade1921/milehigh
#   brew install milehigh
# The Release workflow points url and sha256 at each release's source archive.
class Milehigh < Formula
  desc "Flight log for the terminal, inspired by Flighty, that shares backups with Skyline"
  homepage "https://github.com/lemonade1921/milehigh"
  url "https://github.com/lemonade1921/milehigh/releases/download/v0.1.1/milehigh-0.1.1.tar.gz"
  sha256 "43efa4e485302412b3d20f36f2d3828f63bd49728ec9e51642946598675252ce"
  license "GPL-2.0-or-later"
  head "https://github.com/lemonade1921/milehigh.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/milehigh"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/milehigh version")
    ENV["MILEHIGH_HOME"] = testpath
    system bin/"milehigh", "add", "VY 1234", "2026-11-04", "BCN", "LHR"
    assert_match "\"app\": \"skyline\"", shell_output("#{bin}/milehigh export -")
  end
end
