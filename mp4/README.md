# Parsing and generation of MP4 (isobmff) boxes.

## Background
The basic structure of the code in this directory come from the project https://github.com/jfbus/mp4. It has been vastly enhanced and the focus has changed from progessive mp4 files to segmented files. The code in this directory is therefore partly under the license described in LICENSE.

## Overall structure
Most boxes have their own file named after the box, but in some cases, there may be multiple boxes that have the same content, and the code is then having a generic name like visualsampleentry.go.


The Box interface is specified in `box.go`. It does not contain decode (parsing) methods which have distinct names for each box type
and are dispatched in `box.go`.

To implement a new box `fooo`, the following is needed.

Create a file `fooo.go` and create a struct type `FoooBox`.

Fooo should then implement the Box interface methods:

     Type()
     Size()
     Encode()

but also its own decode method `DecodeFooo`, and register that method in the `decoders` map in `box.go`. For a simple example, look at the `prft` box in `prft.go`.

Container boxes like `moof`, have a list of all their children called `boxes`, but also direct pointers to the children with appropriate names, like `Mfhd` and `Traf`. This makes it easy to chain box paths to reach an element like a TfhdBox like

    file.Moof.Traf.Tfhd

To handle media sample data there are two structures:

1. `Sample` stores the sample information used in trun
2. `SampleComplete` extends this with the sample binary data, and absolute decode and presentation times

A MediaSegment can be fragmented into multiple fragments by the method

    func (s *MediaSegment) Fragmentify(timescale uint64, trex *TrexBox, duration uint32) ([]*Fragment, error)



## License
See [LICENSE.md](LICENSE.md)