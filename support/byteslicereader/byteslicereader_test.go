// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package byteslicereader

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("R", func() {
	var r *R

	BeforeEach(func() {
		r = &R{}
	})

	Context("Read", func() {
		buf := make([]byte, 1024)

		Context("with no data", func() {
			BeforeEach(func() {
				r.Buffer = nil
			})

			It("should read 0 bytes and return EOF", func() {
				v, err := r.Read(buf)

				Expect(v).To(Equal(0))
				Expect(err).To(Equal(io.EOF))
			})
		})

		Context("with one byte of data", func() {
			BeforeEach(func() {
				r.Buffer = []byte{0x7F}
			})

			Context("With a buffer size 0", func() {
				buf := []byte(nil)

				Context("Read", func() {
					It("should read 0 bytes", func() {
						v, err := r.Read(buf)

						Expect(v).To(Equal(0))
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})

		Context("with multiple bytes of data", func() {
			BeforeEach(func() {
				r.Buffer = []byte{0, 1, 2, 3}
			})

			Context("With a larger buffer", func() {
				buf := make([]byte, 1024)

				It("Reads the whole buffer, returning io.EOF", func() {
					v, err := r.Read(buf)

					Expect(v).To(Equal(4))
					Expect(err).To(Equal(io.EOF))
				})
			})

			Context("With a partial read buffer", func() {
				buf := make([]byte, 3)

				It("Reads part of the buffer on first read, remainder on second", func() {
					By("Reads the first part of the buffer")
					v, err := r.Read(buf)
					Expect(v).To(Equal(3))
					Expect(err).ToNot(HaveOccurred())
					Expect(buf[:v]).To(ConsistOf([]byte{0, 1, 2}))

					By("Reads the remainder, returns io.EOF")
					v, err = r.Read(buf)
					Expect(v).To(Equal(1))
					Expect(err).To(Equal(io.EOF))
					Expect(buf[:v]).To(ConsistOf(byte(3)))

					By("Reads again sfter EOF, returns EOF")
					v, err = r.Read(buf)
					Expect(v).To(Equal(0))
					Expect(err).To(Equal(io.EOF))
				})
			})
		})
	})

	Context("ReadByte", func() {
		Context("with no data, should return EOF", func() {
			BeforeEach(func() {
				r.Buffer = nil
			})

			It("should return EOF", func() {
				_, err := r.ReadByte()

				Expect(err).To(Equal(io.EOF))
			})
		})

		Context("with data", func() {
			BeforeEach(func() {
				r.Buffer = []byte{0, 1, 2}
			})

			It("should read the data, then return EOF", func() {
				v, err := r.ReadByte()
				Expect(err).ToNot(HaveOccurred())
				Expect(v).To(Equal(byte(0)))

				v, err = r.ReadByte()
				Expect(err).ToNot(HaveOccurred())
				Expect(v).To(Equal(byte(1)))

				v, err = r.ReadByte()
				Expect(err).ToNot(HaveOccurred())
				Expect(v).To(Equal(byte(2)))

				_, err = r.ReadByte()
				Expect(err).To(Equal(io.EOF))
			})
		})
	})

	Context("Seek", func() {
		Context("using SeekStart", func() {
			Context("with no data", func() {
				BeforeEach(func() {
					r.Buffer = nil
				})

				It("seeking to 0 should return error", func() {
					_, err := r.Seek(0, io.SeekStart)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to positive offset should return error", func() {
					_, err := r.Seek(1337, io.SeekStart)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to negative offset should return error", func() {
					_, err := r.Seek(-1337, io.SeekStart)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("with data", func() {
				BeforeEach(func() {
					r.Buffer = []byte{0, 1, 2, 3}
				})

				It("seeking to 0 should succeed", func() {
					p, err := r.Seek(0, io.SeekStart)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(0)))
				})

				It("seeking to an in-bounds positive offset should succeed and read from there", func() {
					p, err := r.Seek(2, io.SeekStart)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(2)))

					b, err := r.ReadByte()
					Expect(err).ToNot(HaveOccurred())
					Expect(b).To(Equal(byte(2)))
				})

				It("seeking to an out-of-bounds positive offset should fail", func() {
					_, err := r.Seek(1337, io.SeekStart)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to a negative offset should fail", func() {
					_, err := r.Seek(-1, io.SeekStart)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("using SeekEnd", func() {
			Context("with no data", func() {
				BeforeEach(func() {
					r.Buffer = nil
				})

				It("seeking to 0 should return error", func() {
					_, err := r.Seek(0, io.SeekEnd)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to positive offset should succeed, but reads should fail", func() {
					p, err := r.Seek(1337, io.SeekEnd)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(1337)))

					_, err = r.ReadByte()
					Expect(err).To(Equal(io.EOF))
				})

				It("seeking to negative offset should return error", func() {
					_, err := r.Seek(-1337, io.SeekEnd)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("with data", func() {
				BeforeEach(func() {
					r.Buffer = []byte{0, 1, 2, 3}
				})

				It("seeking to 0 should succeed", func() {
					p, err := r.Seek(0, io.SeekEnd)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(3)))
				})

				It("seeking to a positive offset should succeed, but reads should fail", func() {
					p, err := r.Seek(1, io.SeekEnd)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(4)))

					_, err = r.ReadByte()
					Expect(err).To(Equal(io.EOF))
				})

				It("seeking to a in-bounds negative offset should succeed and read from there", func() {
					p, err := r.Seek(-2, io.SeekEnd)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(1)))

					b, err := r.ReadByte()
					Expect(err).ToNot(HaveOccurred())
					Expect(b).To(Equal(byte(1)))
				})

				It("seeking to an out-of-bounds negative offset should fail", func() {
					_, err := r.Seek(-1337, io.SeekEnd)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("using SeekCurrent", func() {
			Context("with no data", func() {
				BeforeEach(func() {
					r.Buffer = nil
				})

				It("seeking to 0 should return error", func() {
					_, err := r.Seek(0, io.SeekCurrent)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to positive offset should return error", func() {
					_, err := r.Seek(1337, io.SeekCurrent)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to negative offset should return error", func() {
					_, err := r.Seek(-1337, io.SeekCurrent)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("with data, after a read", func() {
				BeforeEach(func() {
					r.Buffer = []byte{0, 1, 2, 3}

					_, err := r.ReadByte()
					Expect(err).ToNot(HaveOccurred())
				})

				It("seeking to 0 should succeed", func() {
					p, err := r.Seek(0, io.SeekCurrent)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(1)))

					b, err := r.ReadByte()
					Expect(err).ToNot(HaveOccurred())
					Expect(b).To(Equal(byte(1)))
				})

				It("seeking to an in-bounds positive offset should succeed", func() {
					p, err := r.Seek(1, io.SeekCurrent)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(2)))

					b, err := r.ReadByte()
					Expect(err).ToNot(HaveOccurred())
					Expect(b).To(Equal(byte(2)))
				})

				It("seeking to an out-of-bounds positive offset should fail", func() {
					_, err := r.Seek(1337, io.SeekCurrent)
					Expect(err).To(HaveOccurred())
				})

				It("seeking to an in-bounds negative offset should succeed", func() {
					p, err := r.Seek(-1, io.SeekCurrent)
					Expect(err).ToNot(HaveOccurred())
					Expect(p).To(Equal(int64(0)))

					b, err := r.ReadByte()
					Expect(err).ToNot(HaveOccurred())
					Expect(b).To(Equal(byte(0)))
				})

				It("seeking to an out-of-bounds negative offset should fail", func() {
					_, err := r.Seek(-1337, io.SeekCurrent)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Context("Peek", func() {
		// Zero-Copy, we assert that the returned byte slices ARE the same pointer
		// as the underlying Buffer.
		Context("zero-copy", func() {
			Context("with no data", func() {
				BeforeEach(func() {
					r.Buffer = nil
				})

				Context("peeking 0 bytes", func() {
					It("will return no data", func() {
						Expect(r.Peek(0)).To(BeEmpty())
					})
				})

				Context("peeking many bytes", func() {
					It("will return no data", func() {
						Expect(r.Peek(1337)).To(BeEmpty())
					})
				})
			})

			Context("with data, at an offset", func() {
				BeforeEach(func() {
					r.Buffer = []byte{0, 1, 2, 3}

					_, err := r.Seek(1, io.SeekStart)
					Expect(err).ToNot(HaveOccurred())
				})

				Context("peeking 0 bytes", func() {
					It("will return no data", func() {
						Expect(r.Peek(0)).To(BeEmpty())
					})
				})

				Context("peeking 2 bytes", func() {
					It("will return data and not advance", func() {
						buf := r.Peek(2)
						Expect(buf).To(ConsistOf([]byte{1, 2}))
						Expect(&buf[0]).To(BeIdenticalTo(&r.Buffer[1]))

						v, err := r.ReadByte()
						Expect(err).ToNot(HaveOccurred())
						Expect(v).To(Equal(byte(1)))
					})
				})

				Context("peeking many bytes", func() {
					It("will return data and not advance", func() {
						buf := r.Peek(1337)
						Expect(buf).To(ConsistOf([]byte{1, 2, 3}))
						Expect(&buf[0]).To(BeIdenticalTo(&r.Buffer[1]))

						v, err := r.ReadByte()
						Expect(err).ToNot(HaveOccurred())
						Expect(v).To(Equal(byte(1)))
					})
				})
			})
		})

		// Always-Copy, we assert that the returned byte slices are NOT the same
		// pointer as the underlying Buffer.
		Context("always-copy", func() {
			BeforeEach(func() {
				r.AlwaysCopy = true
			})

			Context("with no data", func() {
				BeforeEach(func() {
					r.Buffer = nil
				})

				Context("peeking 0 bytes", func() {
					It("will return no data", func() {
						Expect(r.Peek(0)).To(BeEmpty())
					})
				})

				Context("peeking many bytes", func() {
					It("will return no data", func() {
						Expect(r.Peek(1337)).To(BeEmpty())
					})
				})
			})

			Context("with data, at an offset", func() {
				BeforeEach(func() {
					r.Buffer = []byte{0, 1, 2, 3}

					_, err := r.Seek(1, io.SeekStart)
					Expect(err).ToNot(HaveOccurred())
				})

				Context("peeking 0 bytes", func() {
					It("will return no data", func() {
						Expect(r.Peek(0)).To(BeEmpty())
					})
				})

				Context("peeking 2 bytes", func() {
					It("will return data and not advance", func() {
						buf := r.Peek(2)
						Expect(buf).To(ConsistOf([]byte{1, 2}))
						Expect(&buf[0]).ToNot(BeIdenticalTo(&r.Buffer[1]))

						v, err := r.ReadByte()
						Expect(err).ToNot(HaveOccurred())
						Expect(v).To(Equal(byte(1)))
					})
				})

				Context("peeking many bytes", func() {
					It("will return data and not advance", func() {
						buf := r.Peek(1337)
						Expect(buf).To(ConsistOf([]byte{1, 2, 3}))
						Expect(&buf[0]).ToNot(BeIdenticalTo(&r.Buffer[1]))

						v, err := r.ReadByte()
						Expect(err).ToNot(HaveOccurred())
						Expect(v).To(Equal(byte(1)))
					})
				})
			})
		})
	})

	Context("PeekByte", func() {
		Context("with no data", func() {
			BeforeEach(func() {
				r.Buffer = nil
			})

			It("will return EOF", func() {
				_, err := r.PeekByte()
				Expect(err).To(Equal(io.EOF))
			})
		})

		Context("with data, at an offset", func() {
			BeforeEach(func() {
				r.Buffer = []byte{0, 1, 2, 3}

				_, err := r.Seek(1, io.SeekStart)
				Expect(err).ToNot(HaveOccurred())
			})

			It("will return data and not advance", func() {
				b, err := r.PeekByte()
				Expect(err).ToNot(HaveOccurred())
				Expect(b).To(Equal(byte(1)))
			})
		})

		Context("with data, at EOF", func() {
			BeforeEach(func() {
				r.Buffer = []byte{0, 1, 2, 3}

				_, err := r.Seek(0, io.SeekEnd)
				Expect(err).ToNot(HaveOccurred())

				// Read the last byte, getting us to EOF.
				_, err = r.ReadByte()
				Expect(err).ToNot(HaveOccurred())
			})

			It("will return EOF", func() {
				_, err := r.PeekByte()
				Expect(err).To(Equal(io.EOF))
			})
		})
	})

	Context("Next", func() {
		Context("with no data", func() {
			BeforeEach(func() {
				r.Buffer = nil
			})

			It("asking for 0 should read 0 bytes and return EOF", func() {
				buf, err := r.Next(1)
				Expect(err).To(Equal(io.EOF))
				Expect(buf).To(BeEmpty())
			})

			It("asking for many bytes should read 0 bytes and return EOF", func() {
				buf, err := r.Next(1337)
				Expect(err).To(Equal(io.EOF))
				Expect(buf).To(BeEmpty())
			})
		})

		Context("with multiple bytes of data", func() {
			BeforeEach(func() {
				r.Buffer = []byte{0, 1, 2, 3}
			})

			// Zero-Copy, we assert that the returned byte slices ARE the same pointer
			// as the underlying Buffer.
			Context("zero-copy", func() {
				It("asking for 0 should read 0 bytes", func() {
					buf, err := r.Next(0)
					Expect(err).ToNot(HaveOccurred())
					Expect(buf).To(BeEmpty())
				})

				It("asking for many bytes should read the full buffer and return EOF", func() {
					buf, err := r.Next(1337)
					Expect(err).To(Equal(io.EOF))
					Expect(buf).To(Equal(r.Buffer))
					Expect(&buf[0]).To(BeIdenticalTo(&r.Buffer[0]))
				})

				It("asking incrementally will return subslices, ending with EOF", func() {
					By("reading incrementally")
					buf, err := r.Next(2)
					Expect(err).ToNot(HaveOccurred())
					Expect(buf).To(Equal(r.Buffer[0:2]))
					Expect(&buf[0]).To(BeIdenticalTo(&r.Buffer[0]))

					buf, err = r.Next(1)
					Expect(err).ToNot(HaveOccurred())
					Expect(buf).To(Equal(r.Buffer[2:3]))
					Expect(&buf[0]).To(BeIdenticalTo(&r.Buffer[2]))

					By("reading last byte should return EOF")
					buf, err = r.Next(1)
					Expect(err).To(Equal(io.EOF))
					Expect(buf).To(Equal(r.Buffer[3:4]))
					Expect(&buf[0]).To(BeIdenticalTo(&r.Buffer[3]))

					By("read at EOF should return EOF")
					buf, err = r.Next(1337)
					Expect(err).To(Equal(io.EOF))
					Expect(buf).To(BeEmpty())
				})
			})

			// Always-Copy, we assert that the returned byte slices are NOT the same
			// pointer as the underlying Buffer.
			Context("always-copy", func() {
				BeforeEach(func() {
					r.AlwaysCopy = true
				})

				It("asking for 0 should read 0 bytes", func() {
					buf, err := r.Next(0)
					Expect(err).ToNot(HaveOccurred())
					Expect(buf).To(BeEmpty())
				})

				It("asking for many bytes should read the full buffer and return EOF", func() {
					buf, err := r.Next(1337)
					Expect(err).To(Equal(io.EOF))
					Expect(buf).To(Equal(r.Buffer))
					Expect(&buf[0]).ToNot(BeIdenticalTo(&r.Buffer[0]))
				})

				It("asking incrementally will return subslices, ending with EOF", func() {
					By("reading incrementally")
					buf, err := r.Next(2)
					Expect(err).ToNot(HaveOccurred())
					Expect(buf).To(Equal(r.Buffer[0:2]))
					Expect(&buf[0]).ToNot(BeIdenticalTo(&r.Buffer[0]))

					buf, err = r.Next(1)
					Expect(err).ToNot(HaveOccurred())
					Expect(buf).To(Equal(r.Buffer[2:3]))
					Expect(&buf[0]).ToNot(BeIdenticalTo(&r.Buffer[2]))

					By("reading last byte should return EOF")
					buf, err = r.Next(1)
					Expect(err).To(Equal(io.EOF))
					Expect(buf).To(Equal(r.Buffer[3:4]))
					Expect(&buf[0]).ToNot(BeIdenticalTo(&r.Buffer[3]))

					By("read at EOF should return EOF")
					buf, err = r.Next(1337)
					Expect(err).To(Equal(io.EOF))
					Expect(buf).To(BeEmpty())
				})
			})
		})
	})

	Context("testing copying", func() {
		BeforeEach(func() {
			r.Buffer = []byte{1, 2, 3, 4}

			_, err := r.Seek(2, io.SeekStart)
			Expect(err).ToNot(HaveOccurred())
		})

		It("maintains state when copied", func() {
			clone := *r

			By("advancing r, to compare")
			b, err := r.ReadByte()
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(byte(3)))

			b, err = r.ReadByte()
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(byte(4)))

			By("checking that clone hasn't moved")
			b, err = clone.ReadByte()
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(byte(3)))

			b, err = clone.ReadByte()
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(byte(4)))
		})
	})
})

func TestR(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testing a byteslicereader.R")
}
