# Homebrew formula. This repository doubles as a tap:
#   brew tap lemonade1921/milehigh https://github.com/lemonade1921/milehigh
#   brew install milehigh
# The Release workflow updates url and sha256 for every tagged version.
class Milehigh < Formula
  desc "Flight log for the terminal, inspired by Flighty, that shares backups with Skyline"
  homepage "https://github.com/lemonade1921/milehigh"
  url "https://github.com/lemonade1921/milehigh/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
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
