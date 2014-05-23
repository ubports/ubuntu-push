/*
 Copyright 2013-2014 Canonical Ltd.

 This program is free software: you can redistribute it and/or modify it
 under the terms of the GNU General Public License version 3, as published
 by the Free Software Foundation.

 This program is distributed in the hope that it will be useful, but
 WITHOUT ANY WARRANTY; without even the implied warranties of
 MERCHANTABILITY, SATISFACTORY QUALITY, or FITNESS FOR A PARTICULAR
 PURPOSE.  See the GNU General Public License for more details.

 You should have received a copy of the GNU General Public License along
 with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// package nih reimplements libnih-dbus's nih_dbus_path's path element
// quoting.
//
// Reimplementing libnih is a wonderful exercise that everybody should persue
// at least thrice.
package nih

import "strconv"

// Quote() takes a byte slice and quotes it á la libnih.
func Quote(s []byte) []byte {
	if len(s) == 0 {
		return []byte{'_'}
	}
	out := make([]byte, 0, 2*len(s))
	for _, c := range s {
		if ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') {
			out = append(out, c)
		} else {
			if c < 16 {
				out = append(out, '_', '0')
			} else {
				out = append(out, '_')
			}
			out = strconv.AppendUint(out, uint64(c), 16)
		}
	}

	return out
}

// Quote() takes a byte slice and undoes the damage done to it by the quoting.
func Unquote(s []byte) []byte {
	out := make([]byte, 0, len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			if len(s) < i+3 {
				break
			}
			num, err := strconv.ParseUint(string(s[i+1:i+3]), 16, 8)
			if err == nil {
				out = append(out, byte(num))
			}
			i += 2
		} else {
			out = append(out, s[i])
		}
	}

	return out
}
