use strict;
use warnings;
use feature qw(say);
use Getopt::Long;
use FindBin;
use lib "$FindBin::Bin/.";
use libdepsgen;

# Example: .go
my $opt_suffix = '';
# Example: $(FOO_BINARY)
my $opt_target = '';

GetOptions("target|t=s" => \$opt_target,
	   "suffix|s=s" => \$opt_suffix)
    or die("Wrong command line args");

if ($opt_target eq '')
{
    die '--target must be specified';
}

say depsgen::generate($opt_target, $opt_suffix, @ARGV);

exit 0;
