class DingtalkWorkspaceCliBeta < Formula
  desc "Automate DingTalk workspace tasks from the terminal (beta channel)"
  homepage "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli"
  version "1.0.57-beta.4"
  license "Apache-2.0"
  keg_only "it is the beta channel and conflicts with dingtalk-workspace-cli"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.57-beta.4/dws-darwin-arm64.tar.gz"
      sha256 "ff363e258d463732e4dc02aa71ac1b5b1f05c25107c32ddcc5ae11d14931a782"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.57-beta.4/dws-darwin-amd64.tar.gz"
      sha256 "3eafcc4c27931b8457f3611a12ce4ae727f7bd65356fbef80958699defa09acf"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.57-beta.4/dws-linux-arm64.tar.gz"
      sha256 "49eabb4d2c419d8d1fb0fcf71d9c318ef49cf3d198431df1bcbaa71589e11383"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.57-beta.4/dws-linux-amd64.tar.gz"
      sha256 "fb3903485fe494e1fadb67ec87b2fef19cf575c72b7df25f0b93f80f8b36273f"
    end
  end

  resource "skills" do
    url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.57-beta.4/dws-skills.zip"
    sha256 "6bd11363dbd2ce79627f49bc80b8a92f9c79fca083a6bd3ff9bc5c4833594fb9"
  end

  def install
    root = Dir["dws-*"].find { |entry| File.directory?(entry) } || "."
    binary = File.join(root, "dws")
    raise "binary not found: #{binary}" unless File.exist?(binary)

    bin.install binary => "dws"

    %w[LICENSE NOTICE README.md CHANGELOG.md].each do |name|
      source = File.join(root, name)
      pkgshare.install source if File.exist?(source)
    end

    skill_dest = pkgshare/"skills/dws"
    skill_dest.mkpath
    resource("skills").stage do
      cp_r(Dir["*"], skill_dest)
    end
  end

  def caveats
    <<~EOS
      Agent Skills are bundled in #{pkgshare}/skills/dws.
      Run `dws skill setup` to install them into your Agent directories.
      This beta is keg-only. Add #{opt_bin} to PATH to use its `dws` binary.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/dws version")
  end
end
