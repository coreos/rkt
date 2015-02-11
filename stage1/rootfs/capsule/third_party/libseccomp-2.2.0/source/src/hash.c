/**
 * Seccomp Library hash code
 *
 * Release under the Public Domain
 * Author: Bob Jenkins <bob_jenkins@burtleburtle.net>
 */

/*
 * lookup3.c, by Bob Jenkins, May 2006, Public Domain.
 *
 * These are functions for producing 32-bit hashes for hash table lookup.
 * jhash_word(), jhash_le(), jhash_be(), mix(), and final() are externally useful
 * functions.  Routines to test the hash are included if SELF_TEST is defined.
 * You can use this free for any purpose.  It's in the public domain.  It has
 * no warranty.
 *
 * You probably want to use jhash_le().  jhash_le() and jhash_be() hash byte
 * arrays.  jhash_le() is is faster than jhash_be() on little-endian machines.
 * Intel and AMD are little-endian machines.
 *
 * If you want to find a hash of, say, exactly 7 integers, do
 *   a = i1;  b = i2;  c = i3;
 *   mix(a,b,c);
 *   a += i4; b += i5; c += i6;
 *   mix(a,b,c);
 *   a += i7;
 *   final(a,b,c);
 *
 * then use c as the hash value.  If you have a variable length array of
 * 4-byte integers to hash, use jhash_word().  If you have a byte array (like
 * a character string), use jhash_le().  If you have several byte arrays, or
 * a mix of things, see the comments above jhash_le().
 *
 * Why is this so big?  I read 12 bytes at a time into 3 4-byte integers, then
 * mix those integers.  This is fast (you can do a lot more thorough mixing
 * with 12*3 instructions on 3 integers than you can with 3 instructions on 1
 * byte), but shoehorning those bytes into integers efficiently is messy.
 */

#include <stdint.h>

#include "arch.h"
#include "hash.h"

#define hashsize(n)	((uint32_t)1<<(n))
#define hashmask(n)	(hashsize(n)-1)
#define rot(x,k)	(((x)<<(k)) | ((x)>>(32-(k))))

/**
 * Mix 3 32-bit values reversibly
 * @param a 32-bit value
 * @param b 32-bit value
 * @param c 32-bit value
 *
 * This is reversible, so any information in (a,b,c) before mix() is still
 * in (a,b,c) after mix().
 *
 * If four pairs of (a,b,c) inputs are run through mix(), or through mix() in
 * reverse, there are at least 32 bits of the output that are sometimes the
 * same for one pair and different for another pair.
 *
 * This was tested for:
 * - pairs that differed by one bit, by two bits, in any combination of top
 *   bits of (a,b,c), or in any combination of bottom bits of (a,b,c).
 * - "differ" is defined as +, -, ^, or ~^.  For + and -, I transformed the
 *   output delta to a Gray code (a^(a>>1)) so a string of 1's (as is commonly
 *   produced by subtraction) look like a single 1-bit difference.
 * - the base values were pseudorandom, all zero but one bit set, or all zero
 *   plus a counter that starts at zero.
 *
 * Some k values for my "a-=c; a^=rot(c,k); c+=b;" arrangement that
 * satisfy this are
 *     4  6  8 16 19  4
 *     9 15  3 18 27 15
 *    14  9  3  7 17  3
 *
 * Well, "9 15 3 18 27 15" didn't quite get 32 bits diffing for "differ"
 * defined as + with a one-bit base and a two-bit delta.  I used
 * http://burtleburtle.net/bob/hash/avalanche.html to choose the operations,
 * constants, and arrangements of the variables.
 *
 * This does not achieve avalanche.  There are input bits of (a,b,c) that fail
 * to affect some output bits of (a,b,c), especially of a.  The most thoroughly
 * mixed value is c, but it doesn't really even achieve avalanche in c.
 *
 * This allows some parallelism.  Read-after-writes are good at doubling the
 * number of bits affected, so the goal of mixing pulls in the opposite
 * direction as the goal of parallelism.  I did what I could.  Rotates seem to
 * cost as much as shifts on every machine I could lay my hands on, and rotates
 * are much kinder to the top and bottom bits, so I used rotates.
 *
 */
#define mix(a,b,c) \
	{ \
		a -= c;  a ^= rot(c, 4);  c += b; \
		b -= a;  b ^= rot(a, 6);  a += c; \
		c -= b;  c ^= rot(b, 8);  b += a; \
		a -= c;  a ^= rot(c,16);  c += b; \
		b -= a;  b ^= rot(a,19);  a += c; \
		c -= b;  c ^= rot(b, 4);  b += a; \
	}

/**
 * Final mixing of 3 32-bit values (a,b,c) into c
 * @param a 32-bit value
 * @param b 32-bit value
 * @param c 32-bit value
 *
 * Pairs of (a,b,c) values differing in only a few bits will usually produce
 * values of c that look totally different.  This was tested for:
 * - pairs that differed by one bit, by two bits, in any combination of top
 *   bits of (a,b,c), or in any combination of bottom bits of (a,b,c).
 * - "differ" is defined as +, -, ^, or ~^.  For + and -, I transformed the
 *   output delta to a Gray code (a^(a>>1)) so a string of 1's (as is commonly
 *   produced by subtraction) look like a single 1-bit difference.
 * - the base values were pseudorandom, all zero but one bit set, or all zero
 *   plus a counter that starts at zero.
 *
 * These constants passed:
 *  14 11 25 16 4 14 24
 *  12 14 25 16 4 14 24
 * and these came close:
 *   4  8 15 26 3 22 24
 *  10  8 15 26 3 22 24
 *  11  8 15 26 3 22 24
 *
 */
#define final(a,b,c) \
	{ \
		c ^= b; c -= rot(b,14); \
		a ^= c; a -= rot(c,11); \
		b ^= a; b -= rot(a,25); \
		c ^= b; c -= rot(b,16); \
		a ^= c; a -= rot(c,4);  \
		b ^= a; b -= rot(a,14); \
		c ^= b; c -= rot(b,24); \
	}

/**
 * Hash an array of 32-bit values
 * @param k the key, an array of uint32_t values
 * @param length the number of array elements
 * @param initval the previous hash, or an arbitrary value
 *
 * This works on all machines.  To be useful, it requires:
 * - that the key be an array of uint32_t's, and
 * - that the length be the number of uint32_t's in the key
 *
 * The function jhash_word() is identical to jhash_le() on little-endian
 * machines, and identical to jhash_be() on big-endian machines, except that
 * the length has to be measured in uint32_ts rather than in bytes.  jhash_le()
 * is more complicated than jhash_word() only because jhash_le() has to dance
 * around fitting the key bytes into registers.
 *
 */
static uint32_t jhash_word(const uint32_t *k, size_t length, uint32_t initval)
{
	uint32_t a, b, c;

	/* set up the internal state */
	a = b = c = 0xdeadbeef + (((uint32_t)length) << 2) + initval;

	/* handle most of the key */
	while (length > 3) {
		a += k[0];
		b += k[1];
		c += k[2];
		mix(a, b, c);
		length -= 3;
		k += 3;
	}

	/* handle the last 3 uint32_t's */
	switch(length) {
	case 3 :
		c += k[2];
	case 2 :
		b += k[1];
	case 1 :
		a += k[0];
		final(a, b, c);
	case 0:
		/* nothing left to add */
		break;
	}

	return c;
}

/**
 * Hash a variable-length key into a 32-bit value
 * @param k the key (the unaligned variable-length array of bytes)
 * @param length the length of the key, counting by bytes
 * @param initval can be any 4-byte value
 *
 * Returns a 32-bit value.  Every bit of the key affects every bit of the
 * return value.  Two keys differing by one or two bits will have totally
 * different hash values.
 *
 * The best hash table sizes are powers of 2.  There is no need to do mod a
 * prime (mod is sooo slow!).  If you need less than 32 bits, use a bitmask.
 * For example, if you need only 10 bits, do:
 *   h = (h & hashmask(10));
 * In which case, the hash table should have hashsize(10) elements.
 *
 * If you are hashing n strings (uint8_t **)k, do it like this:
 *   for (i=0, h=0; i<n; ++i) h = jhash_le( k[i], len[i], h);
 *
 */
static uint32_t jhash_le(const void *key, size_t length, uint32_t initval)
{
	uint32_t a, b, c;
	union {
		const void *ptr;
		size_t i;
	} u;     /* needed for Mac Powerbook G4 */

	/* set up the internal state */
	a = b = c = 0xdeadbeef + ((uint32_t)length) + initval;

	u.ptr = key;
	if ((arch_def_native->endian == ARCH_ENDIAN_LITTLE) &&
	    ((u.i & 0x3) == 0)) {
		/* read 32-bit chunks */
		const uint32_t *k = (const uint32_t *)key;

		while (length > 12) {
			a += k[0];
			b += k[1];
			c += k[2];
			mix(a, b, c);
			length -= 12;
			k += 3;
		}

		/* "k[2]&0xffffff" actually reads beyond the end of the string,
		 * but then masks off the part it's not allowed to read.
		 * Because the string is aligned, the masked-off tail is in the
		 * same word as the rest of the string.  Every machine with
		 * memory protection I've seen does it on word boundaries, so
		 * is OK with this.  But VALGRIND will still catch it and
		 * complain.  The masking trick does make the hash noticably
		 * faster for short strings (like English words). */
#ifndef VALGRIND

		switch(length) {
		case 12:
			c += k[2];
			b += k[1];
			a += k[0];
			break;
		case 11:
			c += k[2] & 0xffffff;
			b += k[1];
			a += k[0];
			break;
		case 10:
			c += k[2] & 0xffff;
			b += k[1];
			a += k[0];
			break;
		case 9 :
			c += k[2] & 0xff;
			b += k[1];
			a += k[0];
			break;
		case 8 :
			b += k[1];
			a += k[0];
			break;
		case 7 :
			b += k[1] & 0xffffff;
			a += k[0];
			break;
		case 6 :
			b += k[1] & 0xffff;
			a += k[0];
			break;
		case 5 :
			b += k[1] & 0xff;
			a += k[0];
			break;
		case 4 :
			a += k[0];
			break;
		case 3 :
			a += k[0] & 0xffffff;
			break;
		case 2 :
			a += k[0] & 0xffff;
			break;
		case 1 :
			a += k[0] & 0xff;
			break;
		case 0 :
			/* zero length strings require no mixing */
			return c;
		}

#else /* make valgrind happy */

		k8 = (const uint8_t *)k;
		switch(length) {
		case 12:
			c += k[2];
			b += k[1];
			a += k[0];
			break;
		case 11:
			c += ((uint32_t)k8[10]) << 16;
		case 10:
			c += ((uint32_t)k8[9]) << 8;
		case 9 :
			c += k8[8];
		case 8 :
			b += k[1];
			a += k[0];
			break;
		case 7 :
			b += ((uint32_t)k8[6]) << 16;
		case 6 :
			b += ((uint32_t)k8[5]) << 8;
		case 5 :
			b += k8[4];
		case 4 :
			a += k[0];
			break;
		case 3 :
			a += ((uint32_t)k8[2]) << 16;
		case 2 :
			a += ((uint32_t)k8[1]) << 8;
		case 1 :
			a += k8[0];
			break;
		case 0 :
			return c;
		}

#endif /* !valgrind */

	} else if ((arch_def_native->endian == ARCH_ENDIAN_LITTLE) &&
		   ((u.i & 0x1) == 0)) {
		/* read 16-bit chunks */
		const uint16_t *k = (const uint16_t *)key;
		const uint8_t  *k8;

		while (length > 12) {
			a += k[0] + (((uint32_t)k[1]) << 16);
			b += k[2] + (((uint32_t)k[3]) << 16);
			c += k[4] + (((uint32_t)k[5]) << 16);
			mix(a, b, c);
			length -= 12;
			k += 6;
		}

		k8 = (const uint8_t *)k;
		switch(length) {
		case 12:
			c += k[4] + (((uint32_t)k[5]) << 16);
			b += k[2] + (((uint32_t)k[3]) << 16);
			a += k[0] + (((uint32_t)k[1]) << 16);
			break;
		case 11:
			c += ((uint32_t)k8[10]) << 16;
		case 10:
			c += k[4];
			b += k[2] + (((uint32_t)k[3]) << 16);
			a += k[0] + (((uint32_t)k[1]) << 16);
			break;
		case 9 :
			c += k8[8];
		case 8 :
			b += k[2] + (((uint32_t)k[3]) << 16);
			a += k[0] + (((uint32_t)k[1]) << 16);
			break;
		case 7 :
			b += ((uint32_t)k8[6]) << 16;
		case 6 :
			b += k[2];
			a += k[0] + (((uint32_t)k[1]) << 16);
			break;
		case 5 :
			b += k8[4];
		case 4 :
			a += k[0] + (((uint32_t)k[1]) << 16);
			break;
		case 3 :
			a += ((uint32_t)k8[2]) << 16;
		case 2 :
			a += k[0];
			break;
		case 1 :
			a += k8[0];
			break;
		case 0 :
			/* zero length requires no mixing */
			return c;
		}

	} else {
		/* need to read the key one byte at a time */
		const uint8_t *k = (const uint8_t *)key;

		while (length > 12) {
			a += k[0];
			a += ((uint32_t)k[1]) << 8;
			a += ((uint32_t)k[2]) << 16;
			a += ((uint32_t)k[3]) << 24;
			b += k[4];
			b += ((uint32_t)k[5]) << 8;
			b += ((uint32_t)k[6]) << 16;
			b += ((uint32_t)k[7]) << 24;
			c += k[8];
			c += ((uint32_t)k[9]) << 8;
			c += ((uint32_t)k[10]) << 16;
			c += ((uint32_t)k[11]) << 24;
			mix(a, b, c);
			length -= 12;
			k += 12;
		}

		switch(length) {
		case 12:
			c += ((uint32_t)k[11]) << 24;
		case 11:
			c += ((uint32_t)k[10]) << 16;
		case 10:
			c += ((uint32_t)k[9]) << 8;
		case 9 :
			c += k[8];
		case 8 :
			b += ((uint32_t)k[7]) << 24;
		case 7 :
			b += ((uint32_t)k[6]) << 16;
		case 6 :
			b += ((uint32_t)k[5]) << 8;
		case 5 :
			b += k[4];
		case 4 :
			a += ((uint32_t)k[3]) << 24;
		case 3 :
			a += ((uint32_t)k[2]) << 16;
		case 2 :
			a += ((uint32_t)k[1]) << 8;
		case 1 :
			a += k[0];
			break;
		case 0 :
			return c;
		}
	}

	final(a, b, c);
	return c;
}

/**
 * Hash a variable-length key into a 32-bit value
 * @param k the key (the unaligned variable-length array of bytes)
 * @param length the length of the key, counting by bytes
 * @param initval can be any 4-byte value
 *
 * This is the same as jhash_word() on big-endian machines.  It is different
 * from jhash_le() on all machines.  jhash_be() takes advantage of big-endian
 * byte ordering.
 *
 */
static uint32_t jhash_be( const void *key, size_t length, uint32_t initval)
{
	uint32_t a, b, c;
	union {
		const void *ptr;
		size_t i;
	} u; /* to cast key to (size_t) happily */

	/* set up the internal state */
	a = b = c = 0xdeadbeef + ((uint32_t)length) + initval;

	u.ptr = key;
	if ((arch_def_native->endian == ARCH_ENDIAN_BIG) &&
	    ((u.i & 0x3) == 0)) {
		/* read 32-bit chunks */
		const uint32_t *k = (const uint32_t *)key;

		while (length > 12) {
			a += k[0];
			b += k[1];
			c += k[2];
			mix(a, b, c);
			length -= 12;
			k += 3;
		}

		/* "k[2]<<8" actually reads beyond the end of the string, but
		 * then shifts out the part it's not allowed to read.  Because
		 * the string is aligned, the illegal read is in the same word
		 * as the rest of the string.  Every machine with memory
		 * protection I've seen does it on word boundaries, so is OK
		 * with this.  But VALGRIND will still catch it and complain.
		 * The masking trick does make the hash noticably faster for
		 * short strings (like English words). */
#ifndef VALGRIND

		switch(length) {
		case 12:
			c += k[2];
			b += k[1];
			a += k[0];
			break;
		case 11:
			c += k[2] & 0xffffff00;
			b += k[1];
			a += k[0];
			break;
		case 10:
			c += k[2] & 0xffff0000;
			b += k[1];
			a += k[0];
			break;
		case 9 :
			c += k[2] & 0xff000000;
			b += k[1];
			a += k[0];
			break;
		case 8 :
			b += k[1];
			a += k[0];
			break;
		case 7 :
			b += k[1] & 0xffffff00;
			a += k[0];
			break;
		case 6 :
			b += k[1] & 0xffff0000;
			a += k[0];
			break;
		case 5 :
			b += k[1] & 0xff000000;
			a += k[0];
			break;
		case 4 :
			a += k[0];
			break;
		case 3 :
			a += k[0] & 0xffffff00;
			break;
		case 2 :
			a += k[0] & 0xffff0000;
			break;
		case 1 :
			a += k[0] & 0xff000000;
			break;
		case 0 :
			/* zero length strings require no mixing */
			return c;
		}

#else  /* make valgrind happy */

		k8 = (const uint8_t *)k;
		switch(length) {
		case 12:
			c += k[2];
			b += k[1];
			a += k[0];
			break;
		case 11:
			c += ((uint32_t)k8[10]) << 8;
		case 10:
			c += ((uint32_t)k8[9]) << 16;
		case 9 :
			c += ((uint32_t)k8[8]) << 24;
		case 8 :
			b += k[1];
			a += k[0];
			break;
		case 7 :
			b += ((uint32_t)k8[6]) << 8;
		case 6 :
			b += ((uint32_t)k8[5]) << 16;
		case 5 :
			b += ((uint32_t)k8[4]) << 24;
		case 4 :
			a += k[0];
			break;
		case 3 :
			a += ((uint32_t)k8[2]) << 8;
		case 2 :
			a += ((uint32_t)k8[1]) << 16;
		case 1 :
			a += ((uint32_t)k8[0]) << 24;
			break;
		case 0 :
			return c;
		}

#endif /* !VALGRIND */

	} else {
		/* need to read the key one byte at a time */
		const uint8_t *k = (const uint8_t *)key;

		while (length > 12) {
			a += ((uint32_t)k[0]) << 24;
			a += ((uint32_t)k[1]) << 16;
			a += ((uint32_t)k[2]) << 8;
			a += ((uint32_t)k[3]);
			b += ((uint32_t)k[4]) << 24;
			b += ((uint32_t)k[5]) << 16;
			b += ((uint32_t)k[6]) << 8;
			b += ((uint32_t)k[7]);
			c += ((uint32_t)k[8]) << 24;
			c += ((uint32_t)k[9]) << 16;
			c += ((uint32_t)k[10]) << 8;
			c += ((uint32_t)k[11]);
			mix(a, b, c);
			length -= 12;
			k += 12;
		}

		switch(length) {
		case 12:
			c += k[11];
		case 11:
			c += ((uint32_t)k[10]) << 8;
		case 10:
			c += ((uint32_t)k[9]) << 16;
		case 9 :
			c += ((uint32_t)k[8]) << 24;
		case 8 :
			b += k[7];
		case 7 :
			b += ((uint32_t)k[6]) << 8;
		case 6 :
			b += ((uint32_t)k[5]) << 16;
		case 5 :
			b += ((uint32_t)k[4]) << 24;
		case 4 :
			a += k[3];
		case 3 :
			a += ((uint32_t)k[2]) << 8;
		case 2 :
			a += ((uint32_t)k[1]) << 16;
		case 1 :
			a += ((uint32_t)k[0]) << 24;
			break;
		case 0 :
			return c;
		}
	}

	final(a, b, c);
	return c;
}

/**
 * Hash a variable-length key into a 32-bit value
 * @param k the key (the unaligned variable-length array of bytes)
 * @param length the length of the key, counting by bytes
 * @param initval can be any 4-byte value
 *
 * A small wrapper function that selects the proper hash function based on the
 * native machine's byte-ordering.
 *
 */
uint32_t jhash(const void *key, size_t length, uint32_t initval)
{
	if (length % sizeof(uint32_t) == 0)
		return jhash_word(key, (length / sizeof(uint32_t)), initval);
	else if (arch_def_native->endian == ARCH_ENDIAN_BIG)
		return jhash_be(key, length, initval);
	else
		return jhash_le(key, length, initval);
}
