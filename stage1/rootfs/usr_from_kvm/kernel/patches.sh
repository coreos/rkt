set -e
for patch_file in patches/????*.patch ; do
    cat $patch_file | (cd $1 ; patch -p1 --forward) || break
done
