# Parsing and generation of MP4 (isobmff) boxes.

## Background
The Box interfaces and some code in this directory is from the project https://github.com/jfbus/mp4. It has been vastly enhanced and the focus has changed from progessive mp4 files to segmented files.

## Overall structure
Most boxes have their own file named after the box, but in some cases, there may be multiple boxes that have the same content, and the code is then having a generic name like visualsampleentry.go.


The Box interface is specified in `box.go`. It does not contain decode (parsing) methods which have distinct names for each box type
and are dispatched in `box.go`.


## Implement a new box
To implement a new box `fooo`, the following is needed.

Create a file `fooo.go` and create a struct type `FoooBox`.

Fooo should then implement the Box interface methods:

     Type()
     Size()
     Encode()
     Info()

but also its own decode method `DecodeFooo`, and register that method in the `decoders` map in `box.go`. For a simple example, look at the `prft` box in `prft.go`.

A test file `fooo_test.go` should have a test using the method `boxDiffAfterEncodeAndDecode`to check that the box information is equal after
encoding and decoding.

Container boxes like `moof`, have a list of all their children called `Children`,
but also direct pointers to the children with appropriate names, like `Mfhd`
and `Traf`. This makes it easy to chain box paths to reach an element like a TfhdBox as

    file.Moof.Traf.Tfhd

When there may be multiple children with the same name, there may be both a
slice `Trafs` with all boxes and `Traf` that points to the first.

To handle media sample data there are two structures:

1. `Sample` stores the sample information used in trun
2. `FullSample` extends this with the sample binary data and absolute decode time

A MediaSegment can be fragmented into multiple fragments by the method

    func (s *MediaSegment) Fragmentify(timescale uint64, trex *TrexBox, duration uint32) ([]*Fragment, error)



## License
See [LICENSE.md](LICENSE.md)