set -e;

# maintain a cached copy of coreos pxe image

if [ -z "${IMG_URL}" -o -z "${ITMP}" -o -z "${V}" ]; then
	exit 1
fi

if [ ${V} -eq 3 ]; then
	set -x
fi

# flatcar gpg signing key:
# $ gpg2 --list-keys --list-options show-unusable-subkeys \
#     --keyid-format SHORT F88CFEDEFF29A5B4D9523864E25D9AED0593B34A
# pub   rsa4096/0593B34A 2018-02-26 [SC]
#       F88CFEDEFF29A5B4D9523864E25D9AED0593B34A
# uid         [ultimate] Flatcar Buildbot (Official Builds) <buildbot@flatcar-linux.org>
# sub   rsa4096/064D542D 2018-02-26 [S] [revoked: 2018-03-14]
# sub   rsa4096/D0FC498C 2018-03-14 [S] [revoked: 2018-09-26]
# sub   rsa4096/896E394F 2018-09-26 [S] [expires: 2019-09-26]
GPG_LONG_ID="E25D9AED0593B34A"
GPG_KEY="-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFqUFawBEACdnSVBBSx3negnGv7Ppf2D6fbIQAHSzUQ+BA5zEG02BS6EKbJh
t5TzEKCRw6hpPC4vAHbiO8B36Y884sSU5Wc4WMiuJ0Z4XZiZ/DAOl5TFfWwhwU0l
SEe/3BWKRtldEs2hM/NLT7A2pLh6gx5NVJNv7PMTDXVuS8AGqIj6eT41r6cPWE67
pQhC1u91saqIOLB1PnWxw/a7go9x8sJBmEVz0/DRS3dw8qlTx/aKSooyaGzZsfAY
L1+a/xst8LG4xfyHBSAuHSqi76LXCdBogU2vgz2V46z29hYRDfQQQGb4hE7UCrLp
EBOVzdQv/vAA9B4FTB+f5a7Vi4pQnM4DBqKaf8XP4wgQWBW439yqna7rKFAW+JIr
/w8YbczTTlJ2FT8v8z5tbMOZ5a6nXAn45YXh5d80CzqEVnaG8Bbavw3WR3jD81BO
0WK+K2FcEXzOtWkkwmcj9PrOKVnBmBv5I+0xtpo9Do0vyONyXPDNH/I4b3xilupN
bWV1SXUu8jpCf/PaNrj7oKHB9Nciv+4lqu/L5YmbaSLBxAvHSsxRpKV53dFtU+sR
kQM5I774B+GnFvhd6k2uMerWFaA1aq7gv0oOm/H5ZkndR5+eS0SAx49OrMbxKkk0
OKzVVxFDJ4pJWyix3dL7CwmewzuI0ZFHCANBKbiILEzDugAD3mEUZxa8lQARAQAB
tD9GbGF0Y2FyIEJ1aWxkYm90IChPZmZpY2lhbCBCdWlsZHMpIDxidWlsZGJvdEBm
bGF0Y2FyLWxpbnV4Lm9yZz6JAk4EEwEIADgWIQT4jP7e/ymltNlSOGTiXZrtBZOz
SgUCWpQVrAIbAwULCQgHAgYVCgkICwIEFgIDAQIeAQIXgAAKCRDiXZrtBZOzSi5G
EACHLSjK24szSj4O8/N9B6TOLnNPJ17At/two/iHfTxrT8lcLM/JQd97wPqH+mVK
hrZ8tCwTZemVeFNXPVy98VYBTjAXscnVh/22DIEYs1wbjD6w8TwgUvzUzpaQJUVu
YlLG3vGAMGaK5FK41BFtsIkar6zaIVy5BPhrA6ASsL9wg9bwSrXT5eKksbaqAZEG
sMiYZxYWzxQHlPu19afxmzBJdVY9YUHEqBYboslGMlLcgErzF7CaiLjDEPkt5Cic
9J3HjIJwlKmVBT6DBdt/tuuzHQntYfPRfOaLVtF/QxRxKNyBtxYndG6k9Vq/cuIN
i5fHpyZ66+9cwswrLISQpAVWa0AW/TENuduj8IU24zCGL7RZVf0jnmALrqkmBTfY
KwtTdpaFle0dC7QP+B27vT/GhBao9KVazfLoAT82bt3hXqjDciAKAstEbqxs75f2
JhIl0HvqyJ47zY/5zphxZlZ+TfqLvJPoEujEUeuEgKm8xmSgtR/49Ysal6ELxbEg
hc6qLINFeSjyRL20aQkeXtQjmZJGuXbUsLBSbVgUOEU+4vvID7EiYyV7X36OmS5N
4SV0MD0bNF578rL4UwhH1WSDSAgkmrfAhgFNof+MlI4qbn39tPiAT9J9dpENay0r
+yd59VhILA3eafkC6m0rtpejx81sDNoSp3UkUS1Qq167ZLkCDQRalBYrARAAsHEO
v6b39tgGxFeheiTnq5j6N+/OjjJyG21x2Y/nSU5lgqPD8DtgKyFlKvP7Xu+BcaZ7
hWjL0scvq0LOyagWdzWx5nNTSLuf8e+ShlcIs3u8kFX8QMddyD5l76S7nTl9kE1S
i2WkO6B4JgzRQCAQyr2B/knfE2wrxPsJsnB1qzRIAXHKvs8ev8bR+FfFSENxI5Jg
DoU3KbcyJ5lMKdVhIhSyGSPi1/emEpbEIv1XYV9l8g4b6Ht5fVsgeYUZbOF/z5Gc
+Kwf3ikGr3KCM/fl06xS/jpqM08Z/Uyei/L8b7tv9Wjop5SXN0yPAr0KIGQdnq5z
GMPf9rkG0Xg47JSQcvDJb0o/Ybi3ND3Mj/Ci8q5UtBgs9PWVBS4JyihKYx2Lb+Wj
+LERdEuv2qRPXO045VgOT5g0Ntlc8EvmX3ulofbM2f1DnPnq3OxuYRIscR/Nv4gi
coNLexv/+mmhdxVJKCSTVPp4SoK4MdBOT0B6pzZjcQBI1ldePQmRZMQgonekUaje
wWy1hp9o+7qJ8yFkkaLTplbZjQtcwfI7cGqpogQmsIzuxCKxb1ze/jed/ApEj8RD
6+RO/qa3R4EGKlSW7FZH20oEDLyFyeOAmSbZ8cqPny6m8egP5naXwWka4aYelObn
5VY6OdX2CJQUuIq8lXue8wOAPpkPB61JnVjQqaUAEQEAAYkCNgQoAQgAIBYhBPiM
/t7/KaW02VI4ZOJdmu0Fk7NKBQJaqVa3Ah0CAAoJEOJdmu0Fk7NK8WMP/R+T//rW
QeuXMlV+l8bHKcbBGWBvvMV5XcsJKDxtzrclPJLqfuBXSDTwqlirXXqlEeI613kE
UWG0b0Ny0K87g9CnkbsJiizGtyQJp2HuMnjRivTd/1V30ACCaK01nbu1/sdOk6Y4
Cimv+mGEgzjcXVXs72p+qqhDEaMgf1GYjDrzVHUnKUNIU8QOG2HRVhpP27bOg9Ao
a9Exdo04w3dXxso3KGeVkEE8dN0rKmHQ67jcCqKogzNlsIujbJkgRbwk/e3BgDWX
ifQSMW4SAAl/PVP7z3h6QoLcYSddOMMYwqP5Oqe4obBaKgVrn705s/Z0pW5nEzFg
38hEoJe+CCXjPl0zjHKQGzhwR/MLWvMf6jO06uvASiJuU/hefVCCek9b5SLn+IPU
J+uLh57F1I7O4ohPWY9+sbrpibx2pcSmcefVMwX/iSt6RNlBITYVQLGN8+/0gcRz
3jGf7m+M8Y7KYrmFxtwPsFejygDr6VVvoUarPPnJSzP+UdPqzUCcxdnV7Ub4QMRl
wUyvnwgnpn0xOsZ/Pdh5gOC06Yrkjbr12DWIpUxy/9z/QR2TeImi02trRKpCh9xw
0bKlsWBt1oUnNnQjnMUB9tmWsF1I6DrO/FUcB+5d7iy+MnPB1LIKS8JokODWIrOq
dg763UZfGbp4EbLlO1vcwIdKC6AGoS6hoyPUiQRyBBgBCAAmFiEE+Iz+3v8ppbTZ
Ujhk4l2a7QWTs0oFAlqUFisCGwIFCQHhM4ACQAkQ4l2a7QWTs0rBdCAEGQEIAB0W
IQQeEA3Xpnem+aUyyfm1HeN3Bk1ULQUCWpQWKwAKCRC1HeN3Bk1ULe4hD/0XLBuo
inLaN2wVQpbjeIEG9Shbaax+BmsuufjiVgNxKEkBg4q6/miCpdpjYmcvv7nNG5uK
zuQ/fnLzgldiVS0G+0BVBelF1FlT85xaI/enIrsvTauGEsfie7/ljrkV//0MFqdB
ZnM680JDVbvl8f2RDBACmz3PoJr8kg3PZwvb028effeTqhZ8zA5ZW5rum0Cn6dOb
v3OrCyQw/aoUvjH65j3T+fr17Em5dYaxNShFxoMBKxSsr+V4opwGEzBRxuoLrzAl
/LcazNAL/CLj+7JBxFj4FL5fB7VQcBEBDFBwg0ropojUeqT8Y2oyygnwLHc4otwV
TNxezToTFucnIq87IAqpTdEe3dHXx1CRJAyIeXxh6j+rYpidiL4CegIczva/xE+P
CqKV1qsGPysD301pXEYy4W1nLuST1tu/xbZCIJdqUwOxsVN5D9UVsFEr4Szfq0QC
14UQzMeXJSdXE2Z1TAnl7381AUC8LoRp55BH5Jih/zrUT1+HrzwdWBZdBJc04f5I
RiZqhZ8Goso5Ki6yFGCEXuitQUyWS0OWkZTX4m2rNIiPMw8PVweQ+yeqwaAapfm7
JX4l3Wa9fRpwK8LLV5/iaXti7IEla51lCCHRn+yM+0XcYI//53qQXVobcaC8Z9uy
LfJCjCtETknO2/uGL+kNyoZ4ykMfIhqOaxZWnqfzD/4kHM+EB4Yuti1kxFmSdnjp
MLEOXNFRoJcvPL7kw6ZMQaWZ96UOdlcL2GiHWAyYThsSjWez+kZ60GuDL+JwfQaR
InavuacP3Dw2eg8/W5XAT/G2EEmA4wuDMXZ07aPa3nJPdlCMcwxQLyHb6ZgModxZ
IHXaX/JEylapdh0j4sQf5P8OvK2Qq212OVuIaZPnjloQDeJqJTzP9iGDaJ3Ne6gM
n6nZ3ZIK1qtJc9WxRtjIOLS2ZdMSB5JWb1gE4nEkvDChbWKfeMpv5ox8G6HJe9Xk
sygGj876vmyAHDwl8zsYMvWeFZONxsahKpDFjXKMcnIpV8ZPfaCT4r4G6x4Qil8u
A1iwCKXo4d+uq3qrRKyhGOE+B+H/5QCGmmfAXhBVsR2aUldK0kx/IVi7HJD1aBRF
k+cpC0+vMw4O4f4qXzm2z5qWHftcB/EBhN+h4+IIDSE+wEtz9OdEpXXbPZ1sd7eS
8K4OjjliG2meTQE/wvn1BNtJVJ2rGQX6moCGx/1FYdLXLROv6hOnBslMVHFRbe+9
OmTFXEDlb6Nh/08PwYdyqk4qXddebALpC0TmyEty8QnjEmL1IhDtMTDVlj/33imb
L0waKqGJ5U3s2fA8VaDZQWL6U/c71xtuVFt6trS4rnsoBzlILPfC1n2wpPvKPEHL
avOKXgf6jXnmSzi5GbnBgbkCDQRaqVbRARAA0R+Z6SrbAI5b8m/j+Q3yc2tc5wDB
i7Hly0SW95ydLkKGaGvHhpLrBM5WwKdtQzF45A9tlyu6iGys5HWPRW3BqMpZrcv8
+2QHyoI2lYM/b0ioai2gSZB+lao955iJyBQ8c+pLSybxwcdaXTb6iBLGReCYXlrL
QL6H+NYw338x8bhRvaDanPQis81GzxtSZgRjtZbAGSvOgq25A3oCTF45O8cfBz+I
FxNaziS7x6lXuqOatv5n3HzffGOz3q1baKsxMRVGx3PdAI/LvRRd9SeBeTpFZQYY
ujCC5K8ds7yxB39Hel5llKnoXLHNm/wLGukXY+PtJVzhtBDL0X3o6OUfsb9tPzwM
oMyA8gRXf94nw2XRT8MMrjGChB7Clfq9AFP3e44D3MaVWbEGOWNG9rQ5s72dk7dF
K416D5cc+BQ8mvllYzZ8gzOgYKnlfVmhqVDAIkFz601+lLRUdK4pD0t1BCmlINSY
EKQNmp0NCSNVCbWWscKvTjboqb76oH/hjnIDqh3GeGdnIJ8vGwUdNN2NBA0rrK8o
+lD1Kc+e6Whe5xORc5krUZYtDCwW6ylRb118rmrHsojxoTH/kGr2IB0po59LT01l
M6KjLfGWrz76jJZmDLQ2gDBZNjuqDV+raHaKpVgUlbTHvmVvumBCm50Haz5w2vbM
txDxVhxU1FdYY00AEQEAAYkCNgQoAQgAIBYhBPiM/t7/KaW02VI4ZOJdmu0Fk7NK
BQJbq1h6Ah0CAAoJEOJdmu0Fk7NKGuAP/0LeLoKVOI8GRiU25bBek4mElKV5YNwU
8QMf75VPnRxklMFGkrPDuVCHVIsOUGo7jF4EHfH8ACgXNsFx8v9pMgsvk4WvfxbY
hepoNNOF/PLsPc125Z3hNq3uJsAMEpijNt8pNXgMvYj6mUKRGuMcIm1KLlczknwU
vtAIWSV+qqpCUL2miVPzp7Y8lexUeB1dsxAiF4btZIJ2i53S72kPMqwLzHdrPxDt
TiIweNz/T5K+C19MDAZ9AVp5qTcPWhQMDnNz3bY/4B2NcAwPJTCRxt7Ne5Ufxpll
3D92jwKZxREBdBPlRq/Qr4JEm4VXOw4QLFoU/WOyRBd4q4aNeFR00J5unZ2zcQ/E
ZL5OvHmkZ2Xl27Cuky1dAnT6hdadjMgWfQB/giXfP8Tu0Qpi7ISv5fEyUh70RpKr
SPdbUIR92IR8Qu862SSZsn7KoywUb2lFYzj6N9c1XORBexgRQgGAMdcT0REXyyS0
bl+9aBRntiw00FkEe7V1+EOLTi40bbddLC0Oatxa35lYg38VYmnhHCrkUl3iCLa/
AlhZmUGXSwmACNRzVRzFPAZMjdql+SEIF0XLYe96sb5twX2aztemy0GMU0ybK3pH
eYrpccUsPRPiHvT4k5TqAA+D1Y1WDjEhidPCbYeyThhAu+lfJiSVn2ex8ESByA/c
/QqOMREjkWlwiQRyBBgBCAAmFiEE+Iz+3v8ppbTZUjhk4l2a7QWTs0oFAlqpVtEC
GwIFCQHhM4ACQAkQ4l2a7QWTs0rBdCAEGQEIAB0WIQSmIfHalsk8Y5UGgy1gNEOh
0PxJjAUCWqlW0QAKCRBgNEOh0PxJjFXaD/0cyALbk6YivbqAMCMXnfBFj5kOoG5T
EGC7quviOVI+U5yNyFzqJtayfaxX3EsF9IjZR4cW58gdcQALS/gGAukexDigoYUz
2h1q2r4zr5pxbj+ez9+fftNDpwp7CmuaB5bzVh1bu8gwVJf4yaSsGubBIgfaysB0
Mzc4eJqIpDFMRQvSOOv7TgzXqAsXQuphoqkB5RuiKtKeugv4qofH5fuM3C/Y4QZ8
edQlTA41KOay1a76xAK85a8qMCjVQVCrepo5+LYXwZAryp4WKIbTSbUNRr5GGgSa
UWBe0/Rz5eqOL3r1YV1WzttWgBLzZUZJqvaYoWtfJGwjxDAFebE+meqtLIh/IDEu
Tc4D3Vge6kCI1jjNDKMZQYf6j1rybKPVzOgkxjCyRcgUI8Y904l9LZ3/BiRV8dY4
nBjWmCYVJPlAVzfDxFwF+A2kKInskPriiYJpFX8MVjy/6GfkJTtMZo1bovSDZZ0n
2MbQ+V3mftV8GkL+RPU5xQ79dPx6Ki81Dh31/T0d8FkEpWLbDy3gc1qgvRWcp6bC
uS1Rg0pf7+ftRYDEW7BBOBzmqfNljolHMWPeZT/1sCs7PmDS+kErZARFm0huMljt
8MNx50KljIVGDUbjOmDaOopTqKFhho/UTTe1Kho3iwTIYIgrzfuCT7t2k0Wx+/NI
y6BcGlPHU/R95gl0D/4yrId19rW5h425bWYmKZ6Ilh+H1zipl5OS0iEllmm4sLcp
Mub2+B+YFU3/EvbF0zkCny2HXy2gyZLhbvNm6Zr4FPW/xfaEnB4OXOOnUbA4+RNf
7bTngPXwhaxN+wQti+Uo0LcwKAU5KIBC9KcT46NirakEu5+5XaU2r+lsa7hlJWfb
17e4tmcOB4QfMTsJu+4DcWJqu+cdtm2N4VcorJCvfw/EffnGaGK0mwRvJp7CZiWi
Vc3T70fH+Rbv6NrgJEFV90XuoetQROwqjBEdbL8iNcuvjWO8j8NSlRKrV+UivP+w
yDf0UCQoMTnFshBM0ZnW+8i/jqsg3kKxs7xuxCZVMfwxzkNb6h/YlbqjRR/hFZ56
Chf1guaCfYJn0vCtdTLWimasemZfcKX7oE9EIbrs8FZcd89FkU0wgrJRscoUAiVP
mbkklT9AvTy7Gp4CCMS8Z22r3Q0d3GgIvFNhakLyDzBKPBf+vJyQEx9SdFIM/Kjv
4grCEjQNrWXXsh8ecurhciHPuiykffmMYyWUzdcc0pQyyyhoYiGbmflGIKx/6M9D
OOW2Q4k7ogubPRLZ/nabZnxJdIbi8WVXgSI2JCuO3+i9dpW+Q9s8F5mPht1QmQnI
ZrA5R/pLRP2oE9x9LDvUPLkQdLIB9RRyTw6D5A1UOI4TuLPOhFpcXqNODjJcO7kC
DQRbq1i2ARAApdwHI9mdWuHcct2tCY4uRFR9m0CliX2vJ3ZOHBmo1wS3HBv0BkAv
zmQwOE5xMDk6i9aN/w6fYii0s1Pfj2cwLz8Iw93icnInk7WGU2KoryWM9+KNGIA+
XOtyobwTh4BHY5ggeYDkdOs7Nrlj1FTlj428NaevU75Cm9xQm6aAZnZZtjSDBTWw
BuSXfFa70kiZzpwKMP/jB8ylWdA74VzkCFfYcdwJHzzrcDS64VRqNhWM/vRFJmLP
wN4MHkAE5RDb4cjGAwkwmZQuDzuk2O9oOukxKd7v/ZUmql4k0qDxi3M9dC3SJJ+O
fVPRlyZ74UVlspgjr5zxSBCerj/aDbVSWWr6JjgeRTQdg6WKhO0+mfmttiANxv/a
fBMDaxys9ee5sJL+WHP62fucD8ukmMEVM0P971U/JBfV8r8VRpy+OENgt6ynJ9dV
4YCdOT2xo42YwkBCYcVOF6iY2YqFd3oDSZARqEk4vr+A2/eNDU37+OBWr8E1pfO7
H6FW4/tVRxYjywat6743e0VTjNbwPGmOFBGc0VuwCJzRsY5dwIi9hlXDGwfNpgzd
tB+ON4BEY4f8ooSYCfHa9G2HeXj/+txxN6Km8Oh8OnQpyfJ6POQQVXX+bUG1W8EC
jNBdoi6m00ZqNVtDsNbdKdWTYYhKtgPUOreGmF75k+LLjiqO4jIE1E0AEQEAAYkE
cgQYAQgAJhYhBPiM/t7/KaW02VI4ZOJdmu0Fk7NKBQJbq1i2AhsCBQkB4TOAAkAJ
EOJdmu0Fk7NKwXQgBBkBCAAdFiEEYozCEpOAZdq047lJqKvwBYluOU8FAlurWLYA
CgkQqKvwBYluOU9wWBAApKMHrxbOqWa0gij3ODcvzpky76y1YWG45iroC55B56X0
XslUpHJno7vTLobV5aJDeXlgaYD2ptn53wW31fTZL/1P0lkyIu30OwYwLvOxaFjT
rsVPCwTz80h6TzsaShFiKirZJhPg5UzC0xfmM4aaQGsoC/Z5pOTyfrYrXgbQPNUJ
f8zagYqpo0WZoG2R2cNwH5VzlJAv/JBB0SdMVgBS7bUXP1eudqn1gmZxw6GUEGU5
5tj4X72ceYHiA+MMlKWsvpwJD9iRsl3yuzcBi8yOA0/jSrXu+5BLGaAAXMyMKETg
+e1ierxZ64yoV+AU6xcKykVzThxG5SoH6NiXsCs0XBOpWxQjfJ4MAeWLfTRMf805
2OSzRsIf1/p2byyTbuApshp//O9c+jbPgEvG7G4VeQdBROY2/46+XR7Q0BrDMom9
Bmk93SSbG9oubYKKALrjJaPIzTieLM3t2zLKZ/RJ6JARYDd6+BMdVNs9QS6Hkwq1
4lIDxz9jqenAXSpnK8fKg2xxzz/UFhoThlY/wlrWP+Sa4FQl1lorcz6Xid+yNoxF
CZw+iWx7FMng0QDM9rtyhAbFkm7JFnDuojVFeNTdTUy+siAZB0cFdP84BkcYugvx
WGM8uYydVOrPlI/nzGomgljIqgzvJm+Crun8eYggmItY53U6xDJmQT7Xrtk7YCa+
0Q/+PRuDorQauvB53mfynLywqxn3h/NyegDrlyq+5Nqsjm3nq0umUSG4/kXMwALy
0h6boyGWR/rkHnLOE1gLQ6fSlpcN8YHtsW6+czpkVH1b+wws/RPg49muTADHeYeM
n5eC0aVrUq7D7IVH+UGILDWJuzq2b+jO/IpXd9kIPlwY/2PFIjwfoSd7W+pjgVXh
6Z+xtWE5mVXnSfxPIXxv/cNd9LtYyT9R6RN7Xu+3hJz/BRp6MUANbdErYD36zERz
GKUO2eJVbOJReevXb24SZzIJkpBF2qwI5dEl8yk12YpGCu75XtFRux3cVhDpdQsx
+/RZGV7Id1X55s4/LiqF5PSEFTB4kZpiY+meq3sKOPT+Ra9BLeur8yo7ftMK13WB
BL2e/mzwfw+s2x1sjWRCuc5KbnK2yTY9ske2hdtAPmVJTDXBO3JWfZj5xKuuc3mp
q7OEd9+gKTiW4PyZfxQIzwXi9BJ6R3+ax7WYR0bi7Gll0910RNFV3MOiLhupIS0Y
BuipB6OgQNFUSjB6vammTd3R+98jIrtWyRDHPmdtgRcK86EbRpj6MHd7rATkdG+S
D0+DXGwfuWIeq2OA+P6lHWEmjlepFSEBS72P5jmpbRtNd+aHN23VesPI/WBQkfBU
4Tu51CGRd4KZk5ugFZ5YqjaM3m70od1zrsdq+BCNsfzuJqU=
=hIuN
-----END PGP PUBLIC KEY BLOCK-----
"

# prints passed flags if verbosity level is lower; to be used inside
# output capture (backticks or $())
function be_quiet() {
	local verbosity	#verbosity level
	local flags	#silencing flags to use

	verbosity=$1
	flags=$2

	if [ $verbosity -lt 3 ]; then
		printf '%s\n' "${flags}"
	fi
}

# gpg verify a file using the provided key
function gpg_verify() {
	local file	#file to verify
	local sigfile	#signature file (assumed to be suffixed form of file to verify)
	local key	#signing key
	local keyid	#signing key signature
	local verbosity	#verbosity level

	local quiet
	local gpghome

	file=$1
	sigfile=$2
	key=$3
	keyid=$4
	verbosity=$5

	quiet=$(be_quiet $verbosity '--quiet --no-verbose')
	gpghome=$(mktemp -d)
	trap "{ rm -rf '${gpghome}'; }" RETURN EXIT
	if ! gpg --homedir="${gpghome}" --batch --quiet --import <<<"${key}"; then
		return 1
	fi
	if ! gpg --homedir="${gpghome}" --batch ${quiet} --trusted-key "${keyid}" --verify "${sigfile}" "${file}"; then
		return 1
	fi
	return 0
}

function do_wget() {
	local out	#output file
	local url	#url of a file to be downloaded
	local verbosity	#verbosity level

	local quiet
	local short_out

	out=$1
	url=$2
	verbosity=$3

	quiet=$(be_quiet $verbosity '--quiet')
	if [ "${quiet}" ]; then
		# strip the working directory from output path, so we get
		# something like build-rkt/tmp/coreos-common/pxe.img
		# instead of /home/foo/projects/rkt/rkt/build-rkt/...
		short_out="${out#${PWD}/}"
		printf '  %-12s %s\n' 'WGET' "${url} => ${short_out}"
	fi
	wget ${quiet} --tries=20 --output-document="${out}" "${url}" # the wget default for retries is 20 times.
}

function cat_to_stderr_if_verbose() {
	local file
	local verbosity

	file=$1
	verbosity=$2

	if [ -z $(be_quiet $verbosity 'empty-if-verbose') ]; then
		cat "${file}" >&2
	fi
}

# maintain an gpg-verified url cache, assumes signature available @ $url.sig
function cache_url() {
	local cache	#verified cache, will be downloaded from the url if bad or missing
	local url	#url of the file to be downloaded
	local key	#key used for verification
	local keyid	#id of a key used for verification
	local verbosity	#verbosity level

	local urlhash
	local sigfile
	local sigurl
	local gpgout

	cache=$1
	url=$2
	key=$3
	keyid=$4
	verbosity=$5

	urlhash=$(echo -n "${url}" | md5sum)
	sigfile="${cache}.${urlhash%% *}.sig"
	sigurl="${url}.sig"

	gpgout=$(mktemp)
	trap "{ rm -f '${gpgout}'; }" RETURN EXIT
	# verify the cached copy if it exists
	if ! gpg_verify "${cache}" "${sigfile}" "${key}" "${keyid}" "${verbosity}" 2>"${gpgout}"; then
		# refresh the cache on failure, and verify it again
		cat_to_stderr_if_verbose "${gpgout}" "${verbosity}"
		do_wget "${cache}" "${url}" "${verbosity}"
		do_wget "${sigfile}" "${sigurl}" "${verbosity}"
		if ! gpg_verify "${cache}" "${sigfile}" "${key}" "${keyid}" "${verbosity}" 2>"${gpgout}"; then
			# print an error if verification failed
			cat "${gpgout}" >&2
			return 1
		fi
		cat_to_stderr_if_verbose "${gpgout}" "${verbosity}"
	else
		cat_to_stderr_if_verbose "${gpgout}" "${verbosity}"
	fi

	# file $cache exists and can be trusted
	touch "${cache}"
}

# cache pxe image
cache_url "${ITMP}/pxe.img" "${IMG_URL}" "${GPG_KEY}" "${GPG_LONG_ID}" "${V}"
