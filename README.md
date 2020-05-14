## Arcam AVR300 RS232 Library and OSC Bridge

This is a home project to automate my old Arcam AVR300 AV Amp. It's got
a serial port on the back that lets you monitor and control it - volume, source
selection, effects.

Basically I want to be able to control it from my phone over Wifi rather than
trying to find the remote all the time. There are many OSC apps for iPhone and
Android. I'm currently using Clean-OSC on iPhone.

## Arcam AVR300 serial port

The port is null modem wired, garrggh. Using a USB RS232 adapter, you also
will need to use a null modem cable to un-null it.

- http://www.arcamupdate.co.uk/Service/AVR350/RS232AVP_R.pdf

## Serial interface notes

On a Mac, hit ctrl-v <enter> for a 0x0d (\r)

    screen /dev/ttyUSB0 38400

    # Mute Zone 1
    PC_.10
    AV_.10

