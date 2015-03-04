/**
 * The "lookup3.c" Hash Implementation from Bob Jenkins
 *
 * Original Author: Bob Jenkins <bob_jenkins@burtleburtle.net>
 * Source: http://burtleburtle.net/bob/c/lookup3.c
 */

/*
 * Original License:
 *
 * These are functions for producing 32-bit hashes for hash table lookup.
 * hashword(), hashlittle(), hashlittle2(), hashbig(), mix(), and final()
 * are externally useful functions.  Routines to test the hash are included
 * if SELF_TEST is defined.  You can use this free for any purpose.  It's in
 * the public domain.  It has no warranty.
 */

#ifndef _HASH_H
#define _HASH_H

#include <inttypes.h>

uint32_t jhash(const void *key, size_t length, uint32_t initval);

#endif

