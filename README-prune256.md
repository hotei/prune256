<center>
# prune256
</center>


## OVERVIEW

prune256 can be used (with the kill option) to delete noise files from directory trees.  The
list of targets is contained in files containing RE2 strings, conventionally called
list.re2, count.re2 and kill.re2 - but can be any filename in reality.  The argument
files are typically produced by running ls256 against a directory tree. 

### Typical usage

```
echo \\\.go > list.re2  # and/or edit to suit needs

ls256 . | tee thistree.256

prune256 -list="list.re2" thistree.256

This should produce a listing of all the files with .go extension in the current directory.
```
-----
```
prune256 -count=count.re2 files.256 [*.256]

This will produce a count of matches per RE2 but not list individual files.
```
Any combination of kill/count/list files is allowed.  Duplicate targets within the
files will be reduced to single instances (must be same RE2 string - not just same meaning)
This allows you to safely cat files together without worrying about efficiency.

The -ReALLy flag is required for deletes to actually work.  Without it the command runs
but nothing is actually deleted.  Highly recommended for testing the effects before
committing.



### option flags

```
	-nice          number in range [0..100] (100 ms increments) to delay between deletes
	-ReALLy		   actually remove(kill) selected files
	-kill=fname    file.re2 with kill targets
	-count=fname   file.re2 with count targets
	-list=fname    file.re2 with list targets	
```

### Installation

If you have a working go installation on a Unix-like OS:

> ```go get github.com/hotei/prune256```

Will copy github.com/hotei/prune256 to the first entry of your $GOPATH

or if go is not installed yet :

> ```cd DestinationDirectory```

> ```git clone https://github.com/hotei/prune256.git```

### Features

* <font color="red">This program can be used to delete files.  It must be used
carefully to avoid unintended consequences.</font> If possible, use it with a
filesystem (ZFS etc) that allows snapshots and restores.
* To increase speed the program keeps the current directory tree file (x.256) in RAM.
* The program uses concurrent searches on multi-core systems.  This helps most
when there are more than RE2s than there are cores. 
* Go implements a very fast [regular expression matcher][2] with [syntax described here][5]

### Limitations

* To increase speed the program keeps the file.256 in RAM.  This can limit the
size of directory tree that can be searched.  If this is a problem the program
can be easily modified to avoid this - but most users will find the speed preferable.
* BUG? Because the program trims whitespace which occurs at the right side of filenames 
some rare cases of pathological filenames that really contain \t | \r | \n at the end
can be missed. This could be avoided by quoting the name but at the cost of rewriting ls256
and all support programs that use *.256  At present only 2 files were found that
display the problem (out of 4e6) and it's easier to just rename those two. Is Gustavo listening?


<!-- ### BUGS -->

### To-Do

* Essential:
 * TBD
* Nice:
 * TBD

### Change Log
* 2014-03-xx Started

### Resources

* [go language reference] [1] 
* [go standard library regexp package docs] [2]
* [Source for program] [3]
* [RE2 Syntax] [5]

[1]: http://golang.org/ref/spec/ "go reference spec"
[2]: http://golang.org/pkg/regexp "go package regexp docs"
[3]: http://github.com/hotei/prune256 "github.com/hotei/prune256"
[4]: http://godoc.org/hotei/prune256 "godoc for prune256"
[5]: https://code.google.com/p/re2/wiki/Syntax "RE2 syntax"

Comments can be sent to <hotei1352@gmail.com> or to user "hotei" at github.com.
License is BSD-two-clause, in file "LICENSE" and also below

License
-------
The 'prune256' go package/program is distributed under the Simplified BSD License:

> Copyright (c) 2014 David Rook. All rights reserved.
> 
> Redistribution and use in source and binary forms, with or without modification, are
> permitted provided that the following conditions are met:
> 
>    1. Redistributions of source code must retain the above copyright notice, this list of
>       conditions and the following disclaimer.
> 
>    2. Redistributions in binary form must reproduce the above copyright notice, this list
>       of conditions and the following disclaimer in the documentation and/or other materials
>       provided with the distribution.
> 
> THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDER ``AS IS'' AND ANY EXPRESS OR IMPLIED
> WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
> FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> OR
> CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
> CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
> SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
> ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
> NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF
> ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// EOF README-prune256.md - this document (c) 2014 David Rook 
