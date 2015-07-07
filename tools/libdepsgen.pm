package depsgen;

use strict;
use warnings;
use File::Basename;

my $template = <<'MAKE_MAGIC';
!!!DEPS_GEN_FILES_IN_FILL!!!

_DEPS_GEN_DIRS_ := !!!DEPS_GEN_DIRS!!!
_DEPS_GEN_EMPTY_ :=
_DEPS_GEN_SPACE_ := $(_DEPS_GEN_EMPTY_) $(_DEPS_GEN_EMPTY_)
_DEPS_GEN_INVALIDATE_TARGET_ := no

define _DEPS_GEN_SUFFIXED_FILES_WILDCARD_
$(eval _DEPS_GEN_WILDCARD_FILES_ := $(wildcard $1/*$2)) \
$(eval _DEPS_GEN_WILDCARD_FILES_ := $(shell stat --format "%n: %F" $(_DEPS_GEN_WILDCARD_FILES_) | grep -e 'regular file$$' | cut -f1 -d:)) \
$(strip $(_DEPS_GEN_WILDCARD_FILES_)) \
$(eval _DEPS_GEN_WILDCARD_FILES_ :=)
endef

define _DEPS_GEN_COMPARE_LIST_
$(eval _DEPS_GEN_VAR_ := _DEPS_GEN_FILES_IN_$(call escape-for-file,$1)) \
$(eval _DEPS_GEN_TF1_ := $(sort $(strip $(call _DEPS_GEN_SUFFIXED_FILES_WILDCARD_,$1,!!!_DEPS_GEN_FILE_SUFFIX!!!)))) \
$(eval _DEPS_GEN_F1_ := $(patsubst $1/%,%,$(_DEPS_GEN_TF1_))) \
$(eval _DEPS_GEN_F1_ := $(patsubst $1/%,%,$(sort $(wildcard $1/*.go)))) \
$(eval _DEPS_GEN_F2_ := $(strip $(sort $($(_DEPS_GEN_VAR_))))) \
$(eval _DEPS_GEN_CF1_ := $(subst $(_DEPS_GEN_SPACE_),-,$(_DEPS_GEN_F1_))) \
$(eval _DEPS_GEN_CF2_ := $(subst $(_DEPS_GEN_SPACE_),-,$(_DEPS_GEN_F2_))) \
$(eval $(if $(filter $(_DEPS_GEN_CF1_),$(_DEPS_GEN_CF2_)),,$(eval _DEPS_GEN_INVALIDATE_TARGET_ := yes))) \
$(eval _DEPS_GEN_VAR_ :=) \
$(eval _DEPS_GEN_TF1_ :=) \
$(eval _DEPS_GEN_F1_ :=) \
$(eval _DEPS_GEN_F2_ :=) \
$(eval _DEPS_GEN_CF1_ :=) \
$(eval _DEPS_GEN_CF2_ :=)
endef

$(foreach d,$(_DEPS_GEN_DIRS_), \
        $(call _DEPS_GEN_COMPARE_LIST_,$d))

ifeq ($(_DEPS_GEN_INVALIDATE_TARGET_),yes)

# invalidate the target
$(call setup-stamp-file,_DEPS_GEN_INVALID_STAMP_,/!!!DEPS_GEN_TARGET!!!-invalidate)
!!!DEPS_GEN_TARGET!!!: $(_DEPS_GEN_INVALID_STAMP_)
$(_DEPS_GEN_INVALID_STAMP_):
	touch "$@"
.INTERMEDIATE: $(_DEPS_GEN_INVALID_STAMP_)
_DEPS_GEN_INVALID_STAMP_ :=

else

!!!DEPS_GEN_TARGET!!!: !!!DEPS_GEN_TARGET_DEPS!!!

endif

!!!DEPS_GEN_FILES_IN_EMPTY!!!
_DEPS_GEN_DIRS_ :=
_DEPS_GEN_EMPTY_ :=
_DEPS_GEN_SPACE_ :=
_DEPS_GEN_INVALIDATE_TARGET_ :=
_DEPS_GEN_COMPARE_LIST_ :=
MAKE_MAGIC

sub escape_path
{
    my ($path) = @_;

    $path =~ s/[-\/.:]/_/gr;
}

sub sort_u
{
    my @strings = @_;
    my %unique = map { $_ => 1 } @strings;

    sort keys %unique;
}

sub generate
{
    my ($target, $suffix, @files) = @_;
    my @all_files = ();
    my @deps_gen_files_in_fill = ();
    my @deps_gen_files_in_empty = ();
    my @deps_gen_dirs = ();
    my $deps_gen_target = $target;
    my @deps_gen_target_deps = ();
    my $deps_gen_file_suffix = $suffix;
    my %dirs = ();

    for my $f (@files)
    {
	my $dir = dirname($f);

	unless (exists($dirs{$dir}))
	{
	    $dirs{$dir} = [$f];
	}
	else
	{
	    my $files = $dirs{$dir};

	    push(@{$files}, $f);
	}
    }
    @deps_gen_dirs = sort keys %dirs;

    for my $dir (@deps_gen_dirs)
    {
	my $escaped = escape_path($dir);
	my @files = sort_u(@{$dirs{$dir}});
	my @filenames = map { basename $_ } @files;
	my $assign = '_DEPS_GEN_FILES_IN_' . $escaped . ' :=';

	push(@all_files, @files);
	push(@deps_gen_files_in_fill, $assign . ' ' . join(' ', @filenames));
	push(@deps_gen_files_in_empty, $assign);
    }

    @deps_gen_target_deps = sort_u(@all_files);

    my $tempstr = '';
    my $deps_template = $template;

    $tempstr = join("\n", @deps_gen_files_in_fill);
    $deps_template =~ s/!!!DEPS_GEN_FILES_IN_FILL!!!/$tempstr/g;

    $tempstr = join(' ', @deps_gen_dirs);
    $deps_template =~ s/!!!DEPS_GEN_DIRS!!!/$tempstr/g;

    $tempstr = $deps_gen_file_suffix;
    $deps_template =~ s/!!!_DEPS_GEN_FILE_SUFFIX!!!/$tempstr/g;

    $tempstr = $deps_gen_target;
    $deps_template =~ s/!!!DEPS_GEN_TARGET!!!/$tempstr/g;

    $tempstr = join(' ', @deps_gen_target_deps);
    $deps_template =~ s/!!!DEPS_GEN_TARGET_DEPS!!!/$tempstr/g;

    $tempstr = join("\n", @deps_gen_files_in_empty);
    $deps_template =~ s/!!!DEPS_GEN_FILES_IN_EMPTY!!!/$tempstr/g;

    return $deps_template;
}
