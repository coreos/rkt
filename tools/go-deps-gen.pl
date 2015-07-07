use strict;
use warnings;
use File::Basename;
use Capture::Tiny 'capture';
use feature qw(say);
use FindBin;
use lib "$FindBin::Bin/.";
use libdepsgen;

sub get_list
{
    my @cmd = @_;
    my ($stdout, $stderr, $status) = capture {
	system(@cmd);
    };

    if ($status != 0)
    {
	say STDERR $stderr;
	exit 1;
    }

    $stdout =~ s/^'\[//;
    $stdout =~ s/\]'$//;

    return split(/\s+/, $stdout);
}

sub go_list
{
    my ($param, $pkg) = @_;
    ('go', 'list', '-f', "'{{$param}}'", $pkg);
}

my %pkgs = ();

sub package_deps
{
    my ($main, $module) = @_;
    my $pkg = $main . '/' . $module;
    my @list = get_list(go_list('.GoFiles', $pkg));

    push(@list, get_list(go_list('.CgoFiles', $pkg)));

    my @files = map { $module . '/' . $_ } @list;
    my @imports = grep {/^$main/} get_list(go_list('.Imports', $pkg));
    my @filenames = map { basename $_ } @files;

    $pkgs{$pkg} = \@filenames;

    for my $f (@imports)
    {
	unless (exists($pkgs{$f}))
	{
	    $f =~ s/$main\///;
	    push(@files, package_deps($main, $f));
	}
    }

    @files
}

if (@ARGV < 3)
{
    say STDERR "too few parameters (expected repo path, package in repo path, make target)";
    exit 1;
}

my ($main, $module, $target) = @ARGV;
my @files = package_deps($main, $module);
my %uniques = map { $_ => 1 } @files;

@files = keys %uniques;
@files = sort @files;

say depsgen::generate($target, '.go', @files);

exit 0;
