echo "Let's try installing samtools & bwa"
cd
apt-get update
apt-get install -y bzip2 wget gcc make autoconf zlib1g-dev libbz2-dev liblzma-dev libncurses5-dev
wget "https://github.com/samtools/samtools/releases/download/1.4/samtools-1.4.tar.bz2"
pwd
tar -xvjf samtools-1.4.tar.bz2
rm samtools-1.4.tar.bz2
cd samtools-1.4
./configure
make
make install
cd
rm -fr samtools-1.4

wget "https://downloads.sourceforge.net/project/bio-bwa/bwa-0.7.15.tar.bz2?r=https%3A%2F%2Fsourceforge.net%2Fprojects%2Fbio-bwa%2Ffiles%2F&ts=1492592278&use_mirror=netcologne" -O bwa-0.7.15.tar.bz2
tar -xvjf bwa-0.7.15.tar.bz2
cd bwa-0.7.15
make
mv bwa /usr/local/bin/
cd
rm -fr bwa*
